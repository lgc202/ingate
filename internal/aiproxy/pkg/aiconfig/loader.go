// Package aiconfig 订阅 Controller 编译的增量资源并提供不可变请求视图。
// 使用 Delta ADS 的逐资源版本恢复，不解释领域引用，不连接模型后端。
// 断线或坏更新保留最近有效配置；显式删除经整批校验后生效，停止时撤回当前指针。
package aiconfig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/util/wait"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

// Loader 串行应用一个共享数据面组的 Delta ADS 响应，不能复制或重复运行。
// 组合根持有 gRPC 连接；Loader 只拥有流、已接受的资源及当前运行视图。
type Loader struct {
	client  discoveryv3.AggregatedDiscoveryServiceClient
	nodeID  string
	logger  *slog.Logger
	started atomic.Bool

	// state 只由订阅循环读写；mu 仅保护构建完成后的 config 指针交接。
	state  resourceState
	mu     sync.RWMutex
	config *Config
}

// NewLoader 保存组合根已构造的发现客户端与已校验的实例标识，不发起请求。
func NewLoader(client discoveryv3.AggregatedDiscoveryServiceClient, nodeID string, logger *slog.Logger) *Loader {
	return &Loader{client: client, nodeID: nodeID, logger: logger}
}

// Run 同步订阅直至取消；失败按有抖动的指数退避重连，不把通信错误解释为删除。
// 每次新流使用已接受版本对账；取消后等待流结束并撤回当前指针，不修改在途请求的视图。
func (l *Loader) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run AI configuration loader: %w", err)
	}

	if !l.started.CompareAndSwap(false, true) {
		return errors.New("AI configuration loader has already run")
	}

	defer l.clear()
	backoff := wait.Backoff{Duration: time.Second, Factor: 2, Jitter: 0.2, Cap: 30 * time.Second, Steps: 6}
	for ctx.Err() == nil {
		accepted, err := l.runStream(ctx)
		if ctx.Err() != nil {
			break
		}

		l.logger.WarnContext(ctx, "AI configuration subscription interrupted; retaining last valid configuration", "err", err)
		if accepted {
			backoff.Duration = time.Second
			backoff.Steps = 6
		}

		timer := time.NewTimer(backoff.Step())
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}

	return fmt.Errorf("run AI configuration loader: %w", ctx.Err())
}

// Capture 只复制当前不可变视图的指针；未完成首次有效同步或已停止时返回 false。
// 请求开始时捕获一次，处理中不得混用另一版本。断线保留已接受视图，合法空配置仍返回 true。
func (l *Loader) Capture() (*Config, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.config, l.config != nil
}

func (l *Loader) runStream(ctx context.Context) (bool, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := l.client.DeltaAggregatedResources(streamCtx)
	if err != nil {
		return false, fmt.Errorf("open AI configuration stream: %w", err)
	}

	versions := make(map[string]string, len(l.state.resources))
	for name, resource := range l.state.resources {
		versions[name] = resource.version
	}

	if err := stream.Send(&discoveryv3.DeltaDiscoveryRequest{
		Node:    &corev3.Node{Id: l.nodeID, Cluster: aidataplanev1.SharedDataPlaneCluster},
		TypeUrl: aidataplanev1.ResourceType, ResourceNamesSubscribe: []string{"*"}, InitialResourceVersions: versions,
	}); err != nil {
		return false, fmt.Errorf("subscribe to AI configuration: %w", err)
	}

	accepted := false
	for {
		response, err := stream.Recv()
		if err != nil {
			return accepted, fmt.Errorf("receive AI configuration: %w", err)
		}

		feedback := &discoveryv3.DeltaDiscoveryRequest{TypeUrl: aidataplanev1.ResourceType, ResponseNonce: response.Nonce}
		if err := l.apply(response); err != nil {
			// 不把资源正文或内部校验内容发回日志与反馈。NACK 后换新流，避免错误版本参与后续增量。
			feedback.ErrorDetail = &rpcstatus.Status{Code: int32(codes.InvalidArgument), Message: "AI configuration batch rejected"}
			if sendErr := stream.Send(feedback); sendErr != nil {
				return accepted, fmt.Errorf("send AI configuration NACK: %w", sendErr)
			}

			// 原始校验错误可能携带配置字段值；订阅日志只保留失败类别。
			return accepted, errors.New("AI configuration batch rejected")
		}

		accepted = true
		if err := stream.Send(feedback); err != nil {
			// 本地已完整应用；即使 ACK 丢失，重连仍报告这些已接受版本。
			return accepted, fmt.Errorf("acknowledge AI configuration: %w", err)
		}
	}
}

func (l *Loader) apply(response *discoveryv3.DeltaDiscoveryResponse) error {
	next, err := l.state.apply(response)
	if err != nil {
		return err
	}

	l.state = *next
	l.mu.Lock()
	defer l.mu.Unlock()
	// 版本可能变化但内容未变；不替换相同入口指针组成的已发布视图。
	if l.config == nil || !maps.Equal(l.config.entries, next.views) {
		l.config = &Config{entries: next.views}
	}

	return nil
}

func (l *Loader) clear() {
	l.state = resourceState{}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = nil
}
