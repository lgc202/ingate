package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lgc202/ingate/internal/aiextproc/biz/tokenquota"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

// ModelAPIKeySource 提供当前已同步的模型 Service API Key。
// 接口位于消费方，具体的数据同步方式不会进入请求处理流程。
type ModelAPIKeySource interface {
	APIKey(serviceID string, protocol aiprotocol.UpstreamProtocol) (string, error)
}

// ExternalProcessor 接收 Envoy 发来的 downstream 和 upstream External Processing 流
// requests 只在单次 HTTP 请求存活期间关联两类流，不承担持久化或跨实例共享。
type ExternalProcessor struct {
	extprocv3.UnimplementedExternalProcessorServer

	mu           sync.RWMutex
	requestsByID map[string]*requestState
	apiKeySource ModelAPIKeySource
	quotaLimiter *tokenquota.Limiter
	logger       *slog.Logger
	counters     processorCounters
}

type processorCounters struct {
	streams atomic.Uint64
	errors  atomic.Uint64
}

// Counters 是 AI ExtProc 运维指标使用的并发安全快照。
type Counters struct {
	Streams            uint64
	Errors             uint64
	ActiveCorrelations int
}

// NewExternalProcessor 创建 External Processing 服务。
func NewExternalProcessor(
	apiKeySource ModelAPIKeySource,
	quotaLimiter *tokenquota.Limiter,
	logger *slog.Logger,
) *ExternalProcessor {
	return &ExternalProcessor{
		requestsByID: make(map[string]*requestState),
		apiKeySource: apiKeySource,
		quotaLimiter: quotaLimiter,
		logger:       logger,
	}
}

// Process 按 Envoy External Processing 协议处理一条双向流
// Envoy 会分别为 downstream filter 和每次 upstream 尝试创建流，二者不是同一个 gRPC stream。
func (p *ExternalProcessor) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	p.counters.streams.Add(1)
	state := streamState{ctx: stream.Context(), processor: p}
	defer state.close()

	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return nil
		}
		if err != nil {
			p.counters.errors.Add(1)
			p.logger.ErrorContext(stream.Context(), "receive ExtProc request failed", "err", err)
			return status.Error(codes.Unavailable, "receive ExtProc request failed")
		}
		if request.GetObservabilityMode() {
			// Observability Mode 明确禁止等待响应，返回消息会被 Envoy 忽略
			continue
		}

		response, err := state.handle(request)
		if err != nil {
			p.counters.errors.Add(1)
			p.logger.ErrorContext(stream.Context(), "process ExtProc exchange failed", "err", err)
			return grpcStatusError(err)
		}
		if err := stream.Send(response); err != nil {
			p.counters.errors.Add(1)
			p.logger.ErrorContext(stream.Context(), "send ExtProc response failed", "err", err)
			return status.Error(codes.Unavailable, "send ExtProc response failed")
		}
	}
}

// Counters 返回 ExtProc 流量和等待 upstream 关联的请求数。
func (p *ExternalProcessor) Counters() Counters {
	p.mu.RLock()
	active := len(p.requestsByID)
	p.mu.RUnlock()
	return Counters{
		Streams:            p.counters.streams.Load(),
		Errors:             p.counters.errors.Load(),
		ActiveCorrelations: active,
	}
}

func (p *ExternalProcessor) registerRequest(identity callerIdentity) (string, *requestState) {
	// 关联 ID 由服务端生成并覆盖同名客户端 Header，避免不同请求串用共享状态
	id := uuid.NewString()
	state := &requestState{identity: identity}
	p.mu.Lock()
	p.requestsByID[id] = state
	p.mu.Unlock()
	return id, state
}

func (p *ExternalProcessor) settleQuota(ctx context.Context, request *requestState, tokens uint64) {
	// Redis 使用有符号整数，Envoy 元数据使用 double。超出共同精确范围的厂商用量
	// 不能用于结算，也不能先消费 Session 而阻止后续有效的最终用量。
	if tokens > uint64(tokenquotaconfig.MaxTokensPerPeriod) {
		p.logger.ErrorContext(
			ctx,
			"settle token quota",
			"err",
			fmt.Errorf("provider token usage %d exceeds the supported range", tokens),
		)
		return
	}
	session := request.takeQuotaSession()
	if session == nil {
		return
	}
	// 厂商已经产生实际费用；客户端断开不能撤销结算。Counter 仍会施加自己的操作超时。
	if err := p.quotaLimiter.Charge(context.WithoutCancel(ctx), session, int64(tokens)); err != nil {
		// 模型请求已经产生实际费用；此时改写成功响应会诱发客户端重试并造成重复消费
		p.logger.ErrorContext(ctx, "settle token quota", "err", err)
	}
}

func (p *ExternalProcessor) findRequest(id string) (*requestState, bool) {
	p.mu.RLock()
	request, ok := p.requestsByID[id]
	p.mu.RUnlock()
	return request, ok
}

func (p *ExternalProcessor) deleteRequest(id string) {
	// downstream 流覆盖完整请求生命周期，它结束后不会再产生新的 upstream 尝试
	p.mu.Lock()
	delete(p.requestsByID, id)
	p.mu.Unlock()
}

func grpcStatusError(err error) error {
	// ExtProc 配置错误与请求协议错误使用不同状态，便于 Envoy 日志定位部署问题
	switch {
	case errors.Is(err, errUnsupportedPhase):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, errRequestNotBuffered), errors.Is(err, errResponseNotBuffered):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, "process ExtProc exchange failed")
	}
}
