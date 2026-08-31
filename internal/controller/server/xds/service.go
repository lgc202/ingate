// Package xds 提供 Envoy 配置发布所需的 ADS 服务。
package xds

import (
	"context"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	sotwv3 "github.com/envoyproxy/go-control-plane/pkg/server/sotw/v3"
)

var _ discoveryv3.AggregatedDiscoveryServiceServer = (*Service)(nil)

// Service 在 Controller 进程内提供 State-of-the-World ADS 协议实现。
type Service struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	sotw sotwv3.Server
}

// NewService 创建共享 Snapshot Cache 和 ACK/NACK 回调的 ADS 服务。
func NewService(watcher cachev3.ConfigWatcher, callbacks sotwv3.Callbacks, logger log.Logger) *Service {
	return &Service{
		sotw: sotwv3.NewServer(
			// ADS stream 的实际生命周期由 gRPC context 管理；此 context 只满足
			// go-control-plane 的进程级服务构造要求
			context.Background(),
			watcher,
			callbacks,
			sotwv3.WithOrderedADS(),
			sotwv3.WithLogger(logger),
		),
	}
}

// StreamAggregatedResources 将 ADS stream 交给 go-control-plane SotW handler。
func (s *Service) StreamAggregatedResources(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer,
) error {
	return s.sotw.StreamHandler(stream, resource.AnyType)
}
