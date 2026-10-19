package xds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"

	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	deltav3 "github.com/envoyproxy/go-control-plane/pkg/server/delta/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Server 在 Manager 生命周期内提供 Delta ADS gRPC 服务。
type Server struct {
	address    string
	publisher  *Publisher
	grpcServer *grpc.Server
	listening  atomic.Bool
}

// adsServer 只实现 Delta ADS；不保留 SotW 兼容入口。
type adsServer struct {
	discoveryv3.UnimplementedAggregatedDiscoveryServiceServer
	server deltav3.Server
}

var (
	_ manager.Runnable                             = (*Server)(nil)
	_ manager.LeaderElectionRunnable               = (*Server)(nil)
	_ discoveryv3.AggregatedDiscoveryServiceServer = (*adsServer)(nil)
)

// NewServer 注册 ADS 服务，但不监听端口或启动 goroutine。
func NewServer(address string, publisher *Publisher, logger logr.Logger) *Server {
	grpcServer := grpc.NewServer()
	// gRPC Server 持有 ADS 流；Manager 取消时 Stop 会结束流及其内部 goroutine。
	discoveryv3.RegisterAggregatedDiscoveryServiceServer(grpcServer, &adsServer{
		server: deltav3.NewServer(context.Background(), publisher.cache, newCallbacks(logger)),
	})

	return &Server{
		address:    address,
		publisher:  publisher,
		grpcServer: grpcServer,
	}
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

// CheckReady 要求 xDS 正在监听且两个缓存均已安装首份有效配置，合法空配置也可就绪。
func (s *Server) CheckReady(_ *http.Request) error {
	if !s.listening.Load() {
		return errors.New("xDS server is not listening")
	}

	return s.publisher.checkReady()
}

// DeltaAggregatedResources 把资源订阅、版本对账和差异发送交给 go-control-plane。
func (s *adsServer) DeltaAggregatedResources(
	stream discoveryv3.AggregatedDiscoveryService_DeltaAggregatedResourcesServer,
) error {
	return s.server.DeltaStreamHandler(stream, resourcev3.AnyType)
}

func serveError(err error) error {
	if err == nil || errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}

	return fmt.Errorf("serve xDS: %w", err)
}
