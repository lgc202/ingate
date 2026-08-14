package server

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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

// partitionBatch 保存同一个 Kafka Topic 和 Partition 的请求记录
//
// ClickHouse 幂等 Token 以 Kafka Partition 为边界，避免不同 Partition 的
// 拉取顺序变化影响常见故障重试
type partitionBatch struct {
	topic     string
	partition int32
	offsets   []int64
	records   []*alsv1.RequestRecord
}

// idempotencyKey 对 Topic、Partition 和有效消息 offset 的精确序列敏感
// 因此只有完全相同的 Kafka 批次才会共用 ClickHouse 去重标识
func (b partitionBatch) idempotencyKey() string {
	digest := sha256.New()
	var number [8]byte
	binary.BigEndian.PutUint32(number[:4], uint32(len(b.topic)))
	_, _ = digest.Write(number[:4])
	_, _ = digest.Write([]byte(b.topic))
	binary.BigEndian.PutUint32(number[:4], uint32(b.partition))
	_, _ = digest.Write(number[:4])
	for _, offset := range b.offsets {
		binary.BigEndian.PutUint64(number[:], uint64(offset))
		_, _ = digest.Write(number[:])
	}
	return hex.EncodeToString(digest.Sum(nil))
}

// Consumer 从 Kafka 批量读取 ALS RequestRecord
//
// Consumer 使用 At Least Once 语义：ClickHouse 成功保存全部 Partition 批次后
// 才提交 Kafka offset。进程在两者之间退出时 Kafka 会重投，Store 负责吸收
// 完全相同批次的重复写入
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
			// Poll 期间禁止的 Rebalance 必须显式放行，即使本轮没有业务消息
			c.client.AllowRebalance()
			continue
		}
		c.received.Add(uint64(len(messages)))

		batches, invalid := requestBatches(messages)
		if invalid > 0 {
			// 无法解析的消息不可能通过重试恢复，提交其 offset 防止毒消息永久阻塞分区
			c.invalid.Add(uint64(invalid))
			c.logger.Warn("invalid request record messages discarded", "count", invalid)
		}
		stored := 0
		for _, batch := range batches {
			if err := c.recorder.Record(ctx, batch.idempotencyKey(), batch.records); err != nil {
				return fmt.Errorf(
					"record Kafka topic %q partition %d: %w",
					batch.topic,
					batch.partition,
					err,
				)
			}
			stored += len(batch.records)
		}
		c.stored.Add(uint64(stored))
		// 所有 Partition 批次入库后再提交，任何中途失败都会让本轮消息整体重投
		if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
			return fmt.Errorf("commit Kafka offsets: %w", err)
		}
		c.client.AllowRebalance()
	}
}

// requestBatches 解码 Kafka 消息，并按 Topic 和 Partition 生成稳定入库批次
//
// 同一 Partition 内保持 Kafka offset 顺序，使相同消息集合重试时
// 生成相同的 ClickHouse 幂等 Token
func requestBatches(messages []*kgo.Record) ([]partitionBatch, int) {
	type topicPartition struct {
		topic     string
		partition int32
	}

	batches := make([]partitionBatch, 0)
	batchIndexes := make(map[topicPartition]int)
	invalid := 0
	for _, message := range messages {
		record := new(alsv1.RequestRecord)
		if err := proto.Unmarshal(message.Value, record); err != nil || !validRecord(record) {
			invalid++
			continue
		}
		key := topicPartition{topic: message.Topic, partition: message.Partition}
		index, exists := batchIndexes[key]
		if !exists {
			index = len(batches)
			batchIndexes[key] = index
			batches = append(batches, partitionBatch{topic: key.topic, partition: key.partition})
		}
		batch := &batches[index]
		batch.offsets = append(batch.offsets, message.Offset)
		batch.records = append(batch.records, record)
	}
	return batches, invalid
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
