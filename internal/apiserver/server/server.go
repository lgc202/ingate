package server

import (
	"context"
	"sync"

	genericapiserver "k8s.io/apiserver/pkg/server"
)

// Server 让 Kubernetes Generic API Server 接入 Kratos 生命周期
type Server struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
	stop             chan struct{}
	done             chan struct{}
	stopOnce         sync.Once
}

// Start 阻塞运行 Generic API Server，直到 Kratos 调用 Stop
func (s *Server) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-s.stop:
			cancel()
		case <-runCtx.Done():
		}
	}()
	defer close(s.done)
	return s.GenericAPIServer.PrepareRun().RunWithContext(runCtx)
}

// Stop 通知 Generic API Server 停止并等待在途请求完成
func (s *Server) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stop) })
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
