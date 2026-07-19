package xds

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	sotwv3 "github.com/envoyproxy/go-control-plane/pkg/server/sotw/v3"
	"google.golang.org/grpc"
)

// Server 在 Controller 进程内只暴露 State-of-the-World ADS，Delta 明确返回未实现
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

// Serve 在已经绑定的 listener 上运行 ADS gRPC 服务
func (s *Server) Serve(ctx context.Context, listener net.Listener, logger *slog.Logger) error {
	grpcServer := grpc.NewServer()
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, s)
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("xDS gRPC server started", "addr", listener.Addr().String())
		serverErr <- grpcServer.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if err == nil {
			return errors.New("xDS gRPC server stopped unexpectedly")
		}
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve xDS gRPC: %w", err)
	case <-ctx.Done():
		// ADS 是长连接，进程退出时直接停止，避免等待客户端主动关闭 stream
		grpcServer.Stop()
		if err := <-serverErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("serve xDS gRPC: %w", err)
		}
		return nil
	}
}

// StreamAggregatedResources 将 ADS stream 交给 go-control-plane SotW handler
func (s *Server) StreamAggregatedResources(stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	return s.sotw.StreamHandler(stream, resource.AnyType)
}
