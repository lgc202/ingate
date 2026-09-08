package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

const topicRefreshInterval = time.Minute

// TopicMonitor 维护 Kafka Topic 可靠性契约的本地缓存，使写入路径和就绪探针无需同步访问 Kafka。
// Kafka 暂时不可达时保留最近一次有效结果，由 Recorder 根据缓存决定直写 Kafka 还是写入磁盘队列。
// 生命周期状态允许 Kratos 的 Start 和 Stop 并发到达而不遗留后台任务。
type TopicMonitor struct {
	reader   biz.TopicReader
	contract *biz.TopicContract
	logger   *slog.Logger
	timeout  time.Duration

	done        chan struct{}
	running     atomic.Bool
	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	stopping    bool

	checkFailed bool
	lastStatus  biz.TopicStatus
}

// NewTopicMonitor 创建 Kafka Topic 契约监测任务。
func NewTopicMonitor(
	config *conf.Data_Kafka,
	reader biz.TopicReader,
	contract *biz.TopicContract,
	logger *slog.Logger,
) *TopicMonitor {
	return &TopicMonitor{
		reader:   reader,
		contract: contract,
		logger:   logger,
		timeout:  config.GetTopicCheckTimeout().AsDuration(),
		done:     make(chan struct{}),
	}
}

// BeforeStart 在 transport 接收请求前完成首次 Topic 检查。
// Kafka 暂时不可达时保持未知状态，由可写的磁盘队列承接新记录。
func (m *TopicMonitor) BeforeStart(ctx context.Context) error {
	m.refresh(ctx)
	return nil
}

// Start 阻塞运行检查循环，由 Kratos App 管理其生命周期。
func (m *TopicMonitor) Start(ctx context.Context) error {
	if !m.running.CompareAndSwap(false, true) {
		return errors.New("topic monitor is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.lifecycleMu.Lock()
	m.cancel = cancel
	stopping := m.stopping
	m.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}

	defer close(m.done)
	ticker := time.NewTicker(topicRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runCtx.Done():
			return nil
		case <-ticker.C:
			m.refresh(runCtx)
		}
	}
}

// Stop 停止检查循环并等待当前一次 Kafka 请求结束。
func (m *TopicMonitor) Stop(ctx context.Context) error {
	m.lifecycleMu.Lock()
	m.stopping = true
	cancel := m.cancel
	m.lifecycleMu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()

	select {
	case <-m.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop topic monitor: %w", ctx.Err())
	}
}

func (m *TopicMonitor) refresh(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	topology, err := m.reader.ReadTopology(checkCtx)
	if err != nil {
		if ctx.Err() == nil && !m.checkFailed {
			m.logger.WarnContext(ctx, "Kafka topic check failed", "err", err)
		}
		m.checkFailed = true
		return
	}

	recovered := m.checkFailed
	m.checkFailed = false
	status := m.contract.Update(topology)
	if !recovered && status == m.lastStatus {
		return
	}

	m.lastStatus = status
	if status.Compliant {
		m.logger.InfoContext(ctx, "Kafka topic satisfies the configured reliability mode",
			"replication_factor", status.ReplicationFactor,
			"min_insync_replicas", status.MinInSyncReplicas,
		)
		return
	}
	m.logger.ErrorContext(ctx, "Kafka topic does not satisfy the configured reliability mode",
		"replication_factor", status.ReplicationFactor,
		"min_insync_replicas", status.MinInSyncReplicas,
	)
}
