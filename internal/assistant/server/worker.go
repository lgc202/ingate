package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	temporalclient "go.temporal.io/sdk/client"
	temporalworker "go.temporal.io/sdk/worker"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

var errWorkerNotRunning = errors.New("temporal worker is not running")

// Worker 将 Temporal Worker 纳入 Kratos 进程生命周期。
type Worker struct {
	logger *slog.Logger
	worker temporalworker.Worker
	fatal  chan error

	mutex    sync.RWMutex
	started  bool
	running  bool
	stopOnce sync.Once
}

// NewWorker 创建轮询指定任务队列的 Temporal Worker。
func NewWorker(
	config *conf.Temporal,
	client temporalclient.Client,
	logger *slog.Logger,
) *Worker {
	worker := &Worker{
		logger: logger,
		fatal:  make(chan error, 1),
	}
	worker.worker = temporalworker.New(client, config.GetTaskQueue(), temporalworker.Options{
		WorkerStopTimeout: config.GetWorkerStopTimeout().AsDuration(),
		OnFatalError: func(err error) {
			select {
			case worker.fatal <- err:
			default:
			}
		},
	})
	return worker
}

// Start 启动 Worker，并在进程取消或 Worker 发生致命错误前保持运行。
func (w *Worker) Start(ctx context.Context) error {
	w.mutex.Lock()
	if w.started {
		w.mutex.Unlock()
		return errors.New("temporal worker already started")
	}
	w.started = true
	w.mutex.Unlock()

	if err := w.worker.Start(); err != nil {
		return fmt.Errorf("start Temporal worker: %w", err)
	}
	w.mutex.Lock()
	w.running = true
	w.mutex.Unlock()
	w.logger.InfoContext(ctx, "Temporal worker started")

	select {
	case <-ctx.Done():
		return nil
	case err := <-w.fatal:
		w.mutex.Lock()
		w.running = false
		w.mutex.Unlock()
		return fmt.Errorf("run Temporal worker: %w", err)
	}
}

// Stop 停止领取新任务，并等待在途任务到达安全退出点。
func (w *Worker) Stop(_ context.Context) error {
	w.mutex.Lock()
	w.running = false
	started := w.started
	w.mutex.Unlock()
	if !started {
		return nil
	}
	w.stopOnce.Do(func() {
		w.worker.Stop()
		w.logger.Info("Temporal worker stopped")
	})
	return nil
}

// Check 确认 Worker 已经启动且尚未进入停止流程。
func (w *Worker) Check(_ context.Context) error {
	w.mutex.RLock()
	running := w.running
	w.mutex.RUnlock()
	if !running {
		return errWorkerNotRunning
	}
	return nil
}
