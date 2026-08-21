// Package service 实现 Envoy External Processing 协议
//
// 同一客户端请求会产生一条 downstream 流和至少一条 upstream 流，服务只负责关联、
// 编排和生成 ExtProc 响应，具体 Chat Completions 协议转换位于 chatcompletion 子包
package service

import (
	"errors"
	"fmt"
	"io"
	"sync"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ExternalProcessor 接收 Envoy 发来的 downstream 和 upstream External Processing 流
// requests 只在单次 HTTP 请求存活期间关联两类流，不承担持久化或跨实例共享
type ExternalProcessor struct {
	extprocv3.UnimplementedExternalProcessorServer

	mu       sync.RWMutex
	requests map[string]*requestState
	apiKeys  ModelAPIKeySource
}

// NewExternalProcessor 创建 External Processing 服务
func NewExternalProcessor(apiKeys ModelAPIKeySource) *ExternalProcessor {
	return &ExternalProcessor{
		requests: make(map[string]*requestState),
		apiKeys:  apiKeys,
	}
}

// Process 按 Envoy External Processing 协议处理一条双向流
// Envoy 会分别为 downstream filter 和每次 upstream 尝试创建流，二者不是同一个 gRPC stream
func (p *ExternalProcessor) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	state := streamState{processor: p}
	defer state.close()

	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return nil
		}
		if err != nil {
			return fmt.Errorf("receive ExtProc request: %w", err)
		}
		if request.GetObservabilityMode() {
			// Observability Mode 明确禁止等待响应，返回消息会被 Envoy 忽略
			continue
		}

		response, err := state.handle(request)
		if err != nil {
			return grpcStatusError(err)
		}
		if err := stream.Send(response); err != nil {
			return fmt.Errorf("send ExtProc response: %w", err)
		}
	}
}

func (p *ExternalProcessor) registerRequest() (string, *requestState) {
	// 关联 ID 由服务端生成并覆盖同名客户端 Header，避免不同请求串用共享状态
	id := uuid.NewString()
	request := &requestState{}
	p.mu.Lock()
	p.requests[id] = request
	p.mu.Unlock()
	return id, request
}

func (p *ExternalProcessor) findRequest(id string) (*requestState, bool) {
	p.mu.RLock()
	request, ok := p.requests[id]
	p.mu.RUnlock()
	return request, ok
}

func (p *ExternalProcessor) deleteRequest(id string) {
	// downstream 流覆盖完整请求生命周期，它结束后不会再产生新的 upstream 尝试
	p.mu.Lock()
	delete(p.requests, id)
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
		return status.Errorf(codes.Internal, "process HTTP exchange: %v", err)
	}
}
