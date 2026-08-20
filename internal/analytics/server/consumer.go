package server

import (
	"context"
	"errors"
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

// Consumer 从 Kafka 批量读取 ALS RequestRecord
//
// Consumer 使用 At Least Once 语义：ClickHouse 成功保存整批请求后才提交
// Kafka offset。进程在两者之间退出时会重投，请求明细依靠稳定事件 ID
// 和 ReplacingMergeTree 最终收敛
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

// NewConsumer 创建使用手动 offset 提交的消费者组成员
//
// BlockRebalanceOnPoll 保证当前 Poll 返回的记录完成入库和 offset 提交前，
// franz-go 不会把对应 Partition 交给其他 Consumer
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
//
// Kratos 在独立 goroutine 中调用 Start，并在停止时调用 Stop。正常取消返回 nil，
// 其他错误交给 Kratos 结束进程，由部署系统按服务策略重启
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
		for _, fetchErr := range fetches.Errors() {
			var groupSessionError *kgo.ErrGroupSession
			if errors.As(fetchErr.Err, &groupSessionError) {
				// Broker 重启或主机休眠可能使成员暂时离开消费组；franz-go 会自动重新加入
				// 这里不能终止进程，否则一次正常的 Rebalance 会让整个查询服务下线
				c.logger.Warn("Kafka consumer group session lost; waiting to rejoin", "error", groupSessionError.Err)
				continue
			}
			var dataLossError *kgo.ErrDataLoss
			if errors.As(fetchErr.Err, &dataLossError) {
				// franz-go 已把消费位置重置到有效 offset，记录异常后继续处理后续消息
				c.logger.Error("Kafka consumer detected data loss", "error", dataLossError)
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

		records, invalid := requestRecords(messages)
		if invalid > 0 {
			// 无法解析的消息不可能通过重试恢复，提交其 offset 防止毒消息永久阻塞分区
			c.invalid.Add(uint64(invalid))
			c.logger.Warn("invalid request record messages discarded", "count", invalid)
		}
		if err := c.recorder.Record(ctx, records); err != nil {
			return fmt.Errorf("record requests: %w", err)
		}
		c.stored.Add(uint64(len(records)))
		// 整批入库后再提交，中途失败会让本轮消息整体重投
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			return fmt.Errorf("commit Kafka offsets: %w", err)
		}
		c.client.AllowRebalance()
	}
}

func requestRecords(messages []*kgo.Record) ([]*alsv1.RequestRecord, int) {
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
	return records, invalid
}

// Stop 停止拉取并等待当前批次处理结束
//
// Stop 可能在 Start 之前被 Kratos 调用，因此通过 start 和 stopping 协调启动竞态
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

// validRecord 校验跨进程协议边界上 ClickHouse 列类型和查询所需的最小字段
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
