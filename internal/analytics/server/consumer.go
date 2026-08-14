package server

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	requestbiz "github.com/lgc202/ingate/internal/analytics/biz/request"
	"github.com/lgc202/ingate/internal/analytics/conf"
	"github.com/lgc202/ingate/pkg/kafkax"
	"github.com/lgc202/ingate/pkg/tlsx"
)

// recordCounters 是当前进程从 Kafka 接收和保存请求记录的累计计数
type recordCounters struct {
	received uint64
	stored   uint64
	invalid  uint64
}

// Consumer 从 Kafka 批量读取 ALS RequestRecord。
type Consumer struct {
	client          *kgo.Client
	recorder        *requestbiz.Recorder
	logger          *slog.Logger
	batchMaxRecords int
	done            chan struct{}
	start           chan struct{}
	cancel          context.CancelFunc
	stopping        atomic.Bool
	startOnce       sync.Once
	stopOnce        sync.Once
	received        atomic.Uint64
	stored          atomic.Uint64
	invalid         atomic.Uint64
}

// NewConsumer 创建使用手动 offset 提交的消费者组成员。
func NewConsumer(
	config *conf.Data_Kafka,
	recorder *requestbiz.Recorder,
	logger *slog.Logger,
) (*Consumer, error) {
	client, err := kafkax.NewClient(kafkax.Config{
		Brokers:     config.GetBrokers(),
		DialTimeout: config.GetDialTimeout().AsDuration(),
		SASL: kafkax.SASL{
			Mechanism: config.GetSasl().GetMechanism(),
			Username:  config.GetSasl().GetUsername(),
			Password:  config.GetSasl().GetPassword(),
		},
		TLS: tlsx.ClientConfig{
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
	return &Consumer{
		client:          client,
		recorder:        recorder,
		logger:          logger,
		batchMaxRecords: int(config.GetBatchMaxRecords()),
		done:            make(chan struct{}),
		start:           make(chan struct{}),
	}, nil
}

// Start 阻塞运行 Kafka 拉取、ClickHouse 入库和 offset 提交循环
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancel = context.WithCancel(ctx)
	c.startOnce.Do(func() { close(c.start) })
	if c.stopping.Load() {
		c.cancel()
	}
	defer close(c.done)
	defer c.client.CloseAllowingRebalance()
	for {
		fetches := c.client.PollRecords(ctx, c.batchMaxRecords)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return nil
		}
		if fetchErrors := fetches.Errors(); len(fetchErrors) > 0 {
			fetchErr := fetchErrors[0]
			return fmt.Errorf(
				"fetch Kafka topic %q partition %d: %w",
				fetchErr.Topic,
				fetchErr.Partition,
				fetchErr.Err,
			)
		}
		messages := fetches.Records()
		if len(messages) == 0 {
			c.client.AllowRebalance()
			continue
		}
		c.received.Add(uint64(len(messages)))

		records := make([]*alsv1.RequestRecord, 0, len(messages))
		invalid := 0
		for _, message := range messages {
			record := new(alsv1.RequestRecord)
			if err := proto.Unmarshal(message.Value, record); err != nil || !validRecord(record) {
				invalid++
				continue
			}
			records = append(records, record)
		}
		if invalid > 0 {
			// 无法解析的消息不可能通过重试恢复，提交其 offset 防止毒消息永久阻塞分区
			c.invalid.Add(uint64(invalid))
			c.logger.Warn("invalid request record messages discarded", "count", invalid)
		}
		if err := c.recorder.Record(ctx, records); err != nil {
			return fmt.Errorf("record requests: %w", err)
		}
		c.stored.Add(uint64(len(records)))
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			return fmt.Errorf("commit Kafka offsets: %w", err)
		}
		c.client.AllowRebalance()
	}
}

// Stop 停止拉取并等待当前批次处理结束
func (c *Consumer) Stop(ctx context.Context) error {
	c.stopping.Store(true)
	c.stopOnce.Do(func() {
		select {
		case <-c.start:
			c.cancel()
		default:
		}
	})
	select {
	case <-c.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (c *Consumer) counters() recordCounters {
	return recordCounters{
		received: c.received.Load(),
		stored:   c.stored.Load(),
		invalid:  c.invalid.Load(),
	}
}

func validRecord(record *alsv1.RequestRecord) bool {
	if record.GetId() == "" || record.GetStartedAt() == nil || record.GetStartedAt().CheckValid() != nil {
		return false
	}
	if record.GetDuration() != nil && (record.GetDuration().CheckValid() != nil || record.GetDuration().AsDuration() < 0) {
		return false
	}
	if record.GetTimeToFirstByte() != nil &&
		(record.GetTimeToFirstByte().CheckValid() != nil || record.GetTimeToFirstByte().AsDuration() < 0) {
		return false
	}
	return record.GetStatusCode() <= 65535 && record.GetUpstreamAttempts() <= 65535
}

// Ping 验证至少一个 Kafka broker 当前可以完成连接和鉴权
func (c *Consumer) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping Kafka: %w", err)
	}
	return nil
}
