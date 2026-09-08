package kafka

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

// Publish 同步等待整批记录得到 Kafka 的最终投递结果。
//
// 一批消息可能部分成功后返回错误，调用方会把整批写入磁盘队列，
// 因此消费者仍需按 RequestRecord.id 去重。
func (c *Client) Publish(ctx context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	startedAt := time.Now()
	messages := make([]*kgo.Record, 0, len(records))
	for _, record := range records {
		value, err := proto.Marshal(record)
		if err != nil {
			result := biz.PublishResult{
				Failed: len(records),
				Class:  biz.PublishPermanent,
				Err:    fmt.Errorf("marshal request record: %w", err),
			}
			c.events.ObserveKafkaPublish(time.Since(startedAt), result.Class)
			return result
		}
		messages = append(messages, &kgo.Record{
			Key:   []byte(record.GetId()),
			Value: value,
			Headers: []kgo.RecordHeader{
				{Key: requestrecord.ContentTypeHeader, Value: []byte(requestrecord.ContentType)},
				{Key: requestrecord.MessageTypeHeader, Value: []byte(requestrecord.MessageType)},
			},
		})
	}

	var result biz.PublishResult
	var isrFailures int
	for _, produced := range c.kafka.ProduceSync(ctx, messages...) {
		if produced.Err == nil {
			result.Confirmed++
			continue
		}

		result.Failed++
		if isISRFailure(produced.Err) {
			isrFailures++
		}
		class := classifyPublishError(produced.Err)
		if class > result.Class {
			result.Class = class
			result.Err = fmt.Errorf("produce request records: %w", produced.Err)
		}
	}

	c.events.ObserveKafkaPublish(time.Since(startedAt), result.Class)
	c.events.AddKafkaISRFailures(isrFailures)
	return result
}

// classifyPublishError 先判断记录是否可能已进入 Kafka，再使用 Kafka 的可重试属性。
// 结果不确定的错误即使可重试，也必须优先归类，因为重试可能产生重复记录。
func classifyPublishError(err error) biz.PublishClass {
	switch {
	case mayHavePublished(err):
		return biz.PublishUncertain
	case errors.Is(err, kgo.ErrMaxBuffered),
		kerr.IsRetriable(err):
		return biz.PublishTemporary
	default:
		return biz.PublishPermanent
	}
}

func mayHavePublished(err error) bool {
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}

	return errors.Is(err, kgo.ErrRecordTimeout) ||
		errors.Is(err, kgo.ErrRecordRetries) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, kerr.RequestTimedOut) ||
		errors.Is(err, kerr.NetworkException) ||
		errors.Is(err, kerr.NotEnoughReplicasAfterAppend)
}

func isISRFailure(err error) bool {
	return errors.Is(err, kerr.NotEnoughReplicas) ||
		errors.Is(err, kerr.NotEnoughReplicasAfterAppend)
}
