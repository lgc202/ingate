package xds

import (
	"context"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	sotwv3 "github.com/envoyproxy/go-control-plane/pkg/server/sotw/v3"
	"google.golang.org/grpc"
)

// Server 只暴露 State-of-the-World ADS，Delta 由 generated server 明确返回未实现
type Server struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	sotw sotwv3.Server
}

var _ discoveryv3.AggregatedDiscoveryServiceServer = (*Server)(nil)

// NewServer 创建启用顺序响应的 SotW ADS server
func NewServer(ctx context.Context, watcher cachev3.ConfigWatcher, callbacks sotwv3.Callbacks, logger log.Logger) *Server {
	return &Server{
		sotw: sotwv3.NewServer(
			ctx,
			watcher,
			callbacks,
			sotwv3.WithOrderedADS(),
			sotwv3.WithLogger(logger),
		),
	}
}

// RegisterServer 将 SotW ADS server 注册到 gRPC registrar
func RegisterServer(registrar grpc.ServiceRegistrar, server *Server) {
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(registrar, server)
}

// StreamAggregatedResources 将 ADS stream 交给 go-control-plane SotW handler
func (s *Server) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	return s.sotw.StreamHandler(stream, resource.AnyType)
}
