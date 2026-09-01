// Package worker 将 Temporal Worker 纳入 Kratos 进程生命周期。
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/wire"
	temporalclient "go.temporal.io/sdk/client"
	temporalworker "go.temporal.io/sdk/worker"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

var (
	errNotRunning = errors.New("temporal worker is not running")

	// ProviderSet 提供进程内 Temporal Worker。
	ProviderSet = wire.NewSet(NewServer)
)

// Server 启动、停止并报告进程内 Temporal Worker 的状态。
type Server struct {
	logger *slog.Logger
	worker temporalworker.Worker
	fatal  chan error

	mutex    sync.RWMutex
	started  bool
	running  bool
	stopOnce sync.Once
}

// NewServer 创建轮询指定任务队列的 Temporal Worker。
func NewServer(
	config *conf.Temporal,
	client temporalclient.Client,
	logger *slog.Logger,
) *Server {
	server := &Server{
		logger: logger,
		fatal:  make(chan error, 1),
	}
	server.worker = temporalworker.New(client, config.GetTaskQueue(), temporalworker.Options{
		WorkerStopTimeout: config.GetWorkerStopTimeout().AsDuration(),
		OnFatalError: func(err error) {
			select {
			case server.fatal <- err:
			default:
			}
		},
	})
	return server
}

// Start 启动 Worker，并在进程取消或 Worker 发生致命错误前保持运行。
func (s *Server) Start(ctx context.Context) error {
	s.mutex.Lock()
	if s.started {
		s.mutex.Unlock()
		return errors.New("temporal worker already started")
	}
	s.started = true
	s.mutex.Unlock()

	if err := s.worker.Start(); err != nil {
		return fmt.Errorf("start Temporal worker: %w", err)
	}
	s.mutex.Lock()
	s.running = true
	s.mutex.Unlock()
	s.logger.InfoContext(ctx, "Temporal worker started")

	select {
	case <-ctx.Done():
		return nil
	case err := <-s.fatal:
		s.mutex.Lock()
		s.running = false
		s.mutex.Unlock()
		return fmt.Errorf("run Temporal worker: %w", err)
	}
}

// Stop 停止领取新任务，并等待在途任务到达安全退出点。
func (s *Server) Stop(_ context.Context) error {
	s.mutex.Lock()
	s.running = false
	started := s.started
	s.mutex.Unlock()
	if !started {
		return nil
	}
	s.stopOnce.Do(func() {
		s.worker.Stop()
		s.logger.Info("Temporal worker stopped")
	})
	return nil
}

// Check 确认 Worker 已经启动且尚未进入停止流程。
func (s *Server) Check(_ context.Context) error {
	s.mutex.RLock()
	running := s.running
	s.mutex.RUnlock()
	if !running {
		return errNotRunning
	}
	return nil
}
