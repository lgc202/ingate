package envoy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	sotwv3 "github.com/envoyproxy/go-control-plane/pkg/server/sotw/v3"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	_ manager.Runnable                             = (*Server)(nil)
	_ manager.LeaderElectionRunnable               = (*Server)(nil)
	_ discoveryv3.AggregatedDiscoveryServiceServer = (*adsServer)(nil)
)

// Server 在 Manager 生命周期内提供 SotW ADS gRPC 服务。
type Server struct {
	address    string
	publisher  *Publisher
	grpcServer *grpc.Server
	listening  atomic.Bool
}

// adsServer 只实现 SotW ADS；Delta RPC 保持 Unimplemented。
type adsServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	server sotwv3.Server
}

// NewServer 注册 ADS 服务，但不监听端口或启动 goroutine。
func NewServer(address string, publisher *Publisher) *Server {
	grpcServer := grpc.NewServer()
	// gRPC Server 持有 ADS 流；Manager 取消时 Stop 会结束流及其内部 goroutine。
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, &adsServer{
		server: sotwv3.NewServer(context.Background(), publisher.cache, nil),
	})
	return &Server{address: address, publisher: publisher, grpcServer: grpcServer}
}

// Start 提供 ADS 服务，直至 Manager 取消或服务异常退出。
func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen for xDS on %q: %w", s.address, err)
	}
	s.listening.Store(true)
	defer s.listening.Store(false)

	// gRPC Serve 阻塞；这一个 goroutine 由 Start 等待，取消时强制结束长连接。
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.grpcServer.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		s.grpcServer.Stop()
		return serveError(err)
	case <-ctx.Done():
		s.grpcServer.Stop()
		return serveError(<-serveErr)
	}
}

// NeedLeaderElection 使 ADS 与共享数据面 Controller 同属活动 Manager。
func (s *Server) NeedLeaderElection() bool {
	return true
}

// CheckReady 要求 xDS 正在监听且已发布首份完整配置。
func (s *Server) CheckReady(_ *http.Request) error {
	if !s.listening.Load() {
		return errors.New("xDS server is not listening")
	}
	if _, err := s.publisher.cache.GetSnapshot(sharedDataPlaneCluster); err != nil {
		return fmt.Errorf("initial xDS configuration is unavailable: %w", err)
	}
	return nil
}

// StreamAggregatedResources 把 SotW ADS 流交给 go-control-plane。
func (s *adsServer) StreamAggregatedResources(
	stream discoveryv3.AggregatedDiscoveryService_StreamAggregatedResourcesServer,
) error {
	return s.server.StreamHandler(stream, resourcev3.AnyType)
}

func serveError(err error) error {
	if err == nil || errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return fmt.Errorf("serve xDS: %w", err)
}
