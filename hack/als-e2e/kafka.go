package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/als/data/diskqueue"
	alskafka "github.com/lgc202/ingate/internal/als/data/kafka"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

const (
	kafkaTopic       = "ingate.request-records"
	kafkaWaitTimeout = 30 * time.Second
)

type lostAckPublisher struct {
	delegate biz.RecordPublisher
	inject   bool
}

func (p *lostAckPublisher) Publish(ctx context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	result := p.delegate.Publish(ctx, records)
	if result.Err != nil || !p.inject {
		return result
	}

	// 该演练只有一个串行写入方，无需为一次性故障标记增加锁。
	p.inject = false
	// 模拟 Broker 已追加消息、但成功确认在返回客户端前丢失。
	// 故障注入留在演练边界，生产 Recorder 仍按真实的不确定结果走 WAL。
	return biz.PublishResult{
		Failed: len(records),
		Class:  biz.PublishUncertain,
		Err:    errors.New("simulated acknowledgement loss"),
	}
}

func waitKafka(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("kafka", flag.ContinueOnError)
	broker := flags.String("broker", "kafka:9092", "Kafka broker")
	recordID := flags.String("id", "", "request record ID")
	count := flags.Int("count", 1, "required occurrences")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *recordID == "" || *count <= 0 {
		return errors.New("a record ID and a positive count are required")
	}

	waitCtx, cancel := context.WithTimeout(ctx, kafkaWaitTimeout)
	defer cancel()
	return consumeUntil(waitCtx, *broker, *recordID, *count)
}

func verifyDuplicate(ctx context.Context, args []string) (err error) {
	flags := flag.NewFlagSet("duplicate", flag.ContinueOnError)
	broker := flags.String("broker", "kafka:9092", "Kafka broker")
	queuePath := flags.String("queue", "", "temporary WAL directory")
	marker := flags.String("marker", "duplicate", "record marker")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *queuePath == "" {
		return errors.New("queue is required")
	}
	ctx, cancel := context.WithTimeout(ctx, kafkaWaitTimeout)
	defer cancel()

	tracer := noop.NewTracerProvider().Tracer("als-e2e")
	events := alsmetrics.NewEventCollector()
	kafkaConfig := &conf.Data_Kafka{
		Brokers:      []string{*broker},
		Topic:        kafkaTopic,
		WriteTimeout: durationpb.New(5 * time.Second),
		DialTimeout:  durationpb.New(time.Second),
	}
	client, err := alskafka.NewClient(kafkaConfig, events, tracer)
	if err != nil {
		return err
	}
	defer client.Close()

	capacityBytes := int64(4 << 20)
	queue, err := diskqueue.NewQueue(&conf.Data_DiskQueue{
		Path:          *queuePath,
		SegmentBytes:  1 << 20,
		CapacityBytes: &capacityBytes,
		Sync:          true,
	}, tracer)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, queue.Close()) }()

	topic := biz.NewTopicContract(biz.ReliabilityDevelopment)
	topic.Update(biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := biz.NewRecorder(
		&lostAckPublisher{delegate: client, inject: true},
		topic,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	startedAt := time.Now().UTC()
	record := &alsv1.RequestRecord{
		Id:          requestrecord.NewID("als-e2e", "duplicate", *marker, startedAt),
		RequestId:   *marker,
		StartedAt:   timestamppb.New(startedAt),
		Method:      "GET",
		Host:        "als-e2e.local",
		Path:        "/duplicate",
		StatusCode:  200,
		EnvoyNodeId: "als-e2e",
		Protocol:    "HTTP/1.1",
	}
	if err := recorder.Write(ctx, []*alsv1.RequestRecord{record}); err != nil {
		return fmt.Errorf("write uncertain record: %w", err)
	}
	result, err := recorder.ReplayBatch(ctx, 1)
	if err != nil {
		return fmt.Errorf("replay uncertain record: %w", err)
	}
	if result != biz.ReplayCommitted {
		return fmt.Errorf("replay uncertain record returned %s", replayResultName(result))
	}
	if err := consumeUntil(ctx, *broker, record.GetId(), 2); err != nil {
		return err
	}

	fmt.Println(record.GetId())
	return nil
}

func consumeUntil(ctx context.Context, broker, recordID string, want int) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(broker),
		kgo.ConsumeTopics(kafkaTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return fmt.Errorf("create Kafka consumer: %w", err)
	}
	defer client.Close()

	count := 0
	for count < want {
		fetches := client.PollFetches(ctx)
		if err := fetches.Err(); err != nil {
			return fmt.Errorf("consume Kafka records: %w", err)
		}
		fetches.EachRecord(func(message *kgo.Record) {
			record := new(alsv1.RequestRecord)
			if proto.Unmarshal(message.Value, record) == nil &&
				record.GetId() == recordID && string(message.Key) == recordID {
				count++
			}
		})
	}
	return nil
}

func replayResultName(result biz.ReplayResult) string {
	switch result {
	case biz.ReplayIdle:
		return "idle"
	case biz.ReplayCommitted:
		return "committed"
	case biz.ReplayRetry:
		return "retry"
	case biz.ReplayPaused:
		return "paused"
	default:
		return "unknown"
	}
}
