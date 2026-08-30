package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"

	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/internal/pkg/kafkaclient"
	"github.com/lgc202/ingate/internal/pkg/tlsconfig"
)

// requestCounters 是当前进程从 Kafka 接收和保存请求记录的累计计数。
type requestCounters struct {
	received uint64
	stored   uint64
	invalid  uint64
}

// RequestConsumer 从 Kafka 批量读取 ALS RequestRecord。
//
// RequestConsumer 使用 At Least Once 语义：ClickHouse 成功保存整批请求后才提交
// Kafka offset。进程在两者之间退出时会重投，Analytics 使用稳定事件 ID 让
// ClickHouse 在物化视图累计前去重，ReplacingMergeTree 继续作为明细的最终保障。
type RequestConsumer struct {
	client          *kgo.Client
	recorder        *requestbiz.Recorder
	logger          *slog.Logger
	batchMaxRecords int
	done            chan struct{}
	running         atomic.Bool
	lifecycleMu     sync.Mutex
	cancel          context.CancelFunc
	stopping        bool
	received        atomic.Uint64
	stored          atomic.Uint64
	invalid         atomic.Uint64
}

// NewRequestConsumer 创建使用手动 offset 提交的消费者组成员。
//
// BlockRebalanceOnPoll 保证当前 Poll 返回的记录完成入库和 offset 提交前，
// franz-go 不会把对应 Partition 交给其他消费者。
func NewRequestConsumer(
	config *conf.Data_Kafka,
	recorder *requestbiz.Recorder,
	logger *slog.Logger,
) (*RequestConsumer, error) {
	client, err := kafkaclient.New(kafkaclient.Config{
		Brokers:     config.GetBrokers(),
		DialTimeout: config.GetDialTimeout().AsDuration(),
		SASL: kafkaclient.SASL{
			Mechanism: config.GetSasl().GetMechanism(),
			Username:  config.GetSasl().GetUsername(),
			Password:  config.GetSasl().GetPassword(),
		},
		TLS: tlsconfig.ClientConfig{
			Enabled:         config.GetTls().GetEnabled(),
			CAFile:          config.GetTls().GetCaFile(),
			CertificateFile: config.GetTls().GetCertFile(),
			PrivateKeyFile:  config.GetTls().GetKeyFile(),
			ServerName:      config.GetTls().GetServerName(),
		},
	},
		kgo.ConsumeTopics(config.GetTopic()),
		kgo.ConsumerGroup(config.GetGroupId()),
		kgo.ClientID(config.GetClientId()),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.FetchMinBytes(config.GetFetchMinBytes()),
		kgo.FetchMaxWait(config.GetFetchMaxWait().AsDuration()),
	)
	if err != nil {
		return nil, err
	}
	return &RequestConsumer{
		client:          client,
		recorder:        recorder,
		logger:          logger,
		batchMaxRecords: int(config.GetBatchMaxRecords()),
		done:            make(chan struct{}),
	}, nil
}

// Start 阻塞运行 Kafka 拉取、ClickHouse 入库和 offset 提交循环。
//
// Kratos 在独立 goroutine 中调用 Start，并在停止时调用 Stop。正常取消返回 nil，
// 其他错误交给 Kratos 结束进程，由部署系统按服务策略重启。
func (c *RequestConsumer) Start(ctx context.Context) error {
	if !c.running.CompareAndSwap(false, true) {
		return errors.New("request consumer is already running")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.lifecycleMu.Lock()
	c.cancel = cancel
	stopping := c.stopping
	c.lifecycleMu.Unlock()
	if stopping {
		cancel()
	}
	defer close(c.done)
	defer c.client.CloseAllowingRebalance()
	for {
		fetches := c.client.PollRecords(runCtx, c.batchMaxRecords)
		if runCtx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		for _, fetchErr := range fetches.Errors() {
			if groupSessionError, ok := errors.AsType[*kgo.ErrGroupSession](fetchErr.Err); ok {
				// Broker 重启或主机休眠可能使成员暂时离开消费组；franz-go 会自动重新加入
				// 这里不能终止进程，否则一次正常的 Rebalance 会让整个查询服务下线
				c.logger.Debug("Kafka consumer group session lost; waiting to rejoin", "err", groupSessionError.Err)
				continue
			}
			if dataLossError, ok := errors.AsType[*kgo.ErrDataLoss](fetchErr.Err); ok {
				// franz-go 已把消费位置重置到有效 offset，记录异常后继续处理后续消息
				c.logger.Error("Kafka consumer detected data loss", "err", dataLossError)
				continue
			}
			return fmt.Errorf(
				"fetch Kafka topic %q partition %d: %w",
				fetchErr.Topic,
				fetchErr.Partition,
				fetchErr.Err,
			)
		}
		messages := fetches.Records()
		if len(messages) == 0 {
			// Poll 期间禁止的 Rebalance 必须显式放行，即使本轮没有业务消息
			c.client.AllowRebalance()
			continue
		}
		c.received.Add(uint64(len(messages)))

		records, invalid := decodeRequestRecords(messages)
		if invalid > 0 {
			// 无法解析的消息不可能通过重试恢复，提交其 offset 防止毒消息永久阻塞分区
			c.invalid.Add(uint64(invalid))
			c.logger.Warn("invalid request record messages discarded", "count", invalid)
		}
		if err := c.recorder.Save(runCtx, records); err != nil {
			if runCtx.Err() != nil {
				return nil
			}
			return fmt.Errorf("record requests: %w", err)
		}
		c.stored.Add(uint64(len(records)))
		// 整批入库后再提交，中途失败会让本轮消息整体重投
		if err := c.client.CommitUncommittedOffsets(runCtx); err != nil {
			if runCtx.Err() != nil {
				return nil
			}
			return fmt.Errorf("commit Kafka offsets: %w", err)
		}
		c.client.AllowRebalance()
	}
}

// Stop 停止拉取并等待当前批次处理结束。
//
// Stop 可能在 Start 之前被 Kratos 调用，stopping 确保稍后启动的消费循环立即退出。
func (c *RequestConsumer) Stop(ctx context.Context) error {
	c.lifecycleMu.Lock()
	c.stopping = true
	cancel := c.cancel
	c.lifecycleMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-c.done:
	case <-ctx.Done():
		return fmt.Errorf("stop request consumer: %w", ctx.Err())
	}
	return nil
}

// Ping 验证至少一个 Kafka broker 当前可以完成连接和鉴权。
func (c *RequestConsumer) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka: %w", err)
	}
	return nil
}

func (c *RequestConsumer) counters() requestCounters {
	return requestCounters{
		received: c.received.Load(),
		stored:   c.stored.Load(),
		invalid:  c.invalid.Load(),
	}
}
