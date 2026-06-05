package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"k8s.io/client-go/tools/cache"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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

func (s *Server) registerEventHandlers() error {
	_, err := s.factory.Gateway().V1().RuntimeSnapshots().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			s.applySnapshotObject(obj)
		},
		UpdateFunc: func(oldObj, newObj any) {
			s.applySnapshotObject(newObj)
		},
		DeleteFunc: func(obj any) {
			s.deleteSnapshotObject(obj)
		},
	})
	return err
}

func (s *Server) waitForCacheSync(ctx context.Context) error {
	for _, synced := range s.factory.WaitForCacheSync(ctx.Done()) {
		if synced {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("cache sync failed")
	}
	return nil
}

func (s *Server) applySnapshotObject(obj any) {
	snapshot, ok := objectAs[*resource.RuntimeSnapshot](obj)
	if !ok || !s.store.Apply(snapshot) {
		return
	}

	s.logger.Info("runtime snapshot updated",
		"target", snapshot.Spec.Target,
		"gateway", snapshot.Spec.Gateway,
		"version", snapshot.Spec.Version,
	)
	s.ads.NotifySnapshotsChanged()
}

func (s *Server) deleteSnapshotObject(obj any) {
	snapshot, ok := objectAs[*resource.RuntimeSnapshot](obj)
	if !ok || !s.store.Delete(snapshot) {
		return
	}

	s.logger.Info("runtime snapshot removed",
		"target", snapshot.Spec.Target,
		"gateway", snapshot.Spec.Gateway,
	)
	s.ads.NotifySnapshotsChanged()
}

func objectAs[T any](obj any) (T, bool) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	value, ok := obj.(T)
	return value, ok
}
