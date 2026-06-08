package server

import (
	"context"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"

	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
	informers "github.com/lgc202/ingate/pkg/generated/informers/externalversions"
)

// Server 维护 RuntimeSnapshot 观察状态，后续在此基础上提供 xDS 协议
type Server struct {
	factory       informers.SharedInformerFactory
	grpcServer    *grpc.Server
	listenAddress string
	ads           *adsServer
	store         *snapshotStore
	logger        *slog.Logger
}

// New 创建 xDS 配置观察服务
func New(client clientset.Interface, listenAddress, target string, resyncPeriod time.Duration, logger *slog.Logger) *Server {
	store := newSnapshotStore(target)
	server := &Server{
		factory:       informers.NewSharedInformerFactory(client, resyncPeriod),
		grpcServer:    grpc.NewServer(),
		listenAddress: listenAddress,
		store:         store,
		logger:        logger,
	}
	server.ads = registerADSServer(server.grpcServer, store, logger)
	return server
}

// Run 启动 RuntimeSnapshot watch 主循环
func (s *Server) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		s.grpcServer.GracefulStop()
		s.factory.Shutdown()
	}()

	if err := s.registerEventHandlers(); err != nil {
		return err
	}
	s.factory.Start(runCtx.Done())
	if err := s.waitForCacheSync(runCtx); err != nil {
		return err
	}

	s.logger.Info("runtime snapshot watch started", "target", s.store.target)
	listener, err := net.Listen("tcp", s.listenAddress)
	if err != nil {
		return err
	}
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		s.logger.Info("xds grpc server started", "addr", listener.Addr().String())
		serverErr <- s.grpcServer.Serve(listener)
	}()

	select {
	case <-runCtx.Done():
		return runCtx.Err()
	case err := <-serverErr:
		return err
	}
}
