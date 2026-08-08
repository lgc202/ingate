package data

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	accessKeySyncInterval = 30 * time.Second
	accessKeySyncTimeout  = 10 * time.Second
)

// AccessKeyIndexSync 周期性用 MySQL 事实修复 Redis 访问密钥执行索引
type AccessKeyIndexSync struct {
	repository *accessKeyRepository
	logger     *slog.Logger
	stop       chan struct{}
	done       chan struct{}
	stopOnce   sync.Once
}

// NewAccessKeyIndexSync 创建由 Kratos App 托管生命周期的访问密钥同步任务
func NewAccessKeyIndexSync(repository *accessKeyRepository, logger *slog.Logger) *AccessKeyIndexSync {
	return &AccessKeyIndexSync{
		repository: repository,
		logger:     logger,
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// Reconcile 在服务接收请求前完成一次全量同步
func (s *AccessKeyIndexSync) Reconcile(ctx context.Context) error {
	return s.repository.Reconcile(ctx)
}

// Start 定期修复跨存储写入或 Redis 重启造成的执行索引差异
func (s *AccessKeyIndexSync) Start(ctx context.Context) error {
	defer close(s.done)
	ticker := time.NewTicker(accessKeySyncInterval)
	defer ticker.Stop()

	failed := false
	for {
		select {
		case <-ticker.C:
			reconcileCtx, cancel := context.WithTimeout(ctx, accessKeySyncTimeout)
			err := s.Reconcile(reconcileCtx)
			cancel()
			if err != nil {
				s.logger.ErrorContext(ctx, "access key index reconciliation failed", "err", err)
				failed = true
				continue
			}
			if failed {
				s.logger.InfoContext(ctx, "access key index reconciliation recovered")
				failed = false
			}
		case <-s.stop:
			return nil
		}
	}
}

// Stop 停止同步任务并等待后台循环退出
func (s *AccessKeyIndexSync) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stop) })
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
