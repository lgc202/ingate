package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
)

// TestPublishAggregatesRecordFailures 验证同步发布保留每条记录的最终结果。
func TestPublishAggregatesRecordFailures(t *testing.T) {
	kafka, err := kgo.NewClient(
		kgo.SeedBrokers("127.0.0.1:1"),
		kgo.DefaultProduceTopic("request-records"),
	)
	if err != nil {
		t.Fatalf("kgo.NewClient() error = %v, want nil", err)
	}
	t.Cleanup(kafka.Close)
	publisher := &Client{
		kafka:  kafka,
		topic:  "request-records",
		events: alsmetrics.NewEventCollector(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := publisher.Publish(ctx, []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}})
	if result.Confirmed != 0 || result.Failed != 2 {
		t.Errorf("Client.Publish() counts = (%d confirmed, %d failed), want (0, 2)", result.Confirmed, result.Failed)
	}
	if result.Class != biz.PublishUncertain || !errors.Is(result.Err, context.Canceled) {
		t.Errorf("Client.Publish() failure = (%v, %v), want uncertain context cancellation", result.Class, result.Err)
	}
}

// TestPublishClass 验证 Kafka 投递错误的重试语义保持稳定。
func TestPublishClass(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want biz.PublishClass
		isr  bool
	}{
		{name: "buffer full", err: kgo.ErrMaxBuffered, want: biz.PublishTemporary},
		{name: "leader unavailable", err: kerr.LeaderNotAvailable, want: biz.PublishTemporary},
		{name: "below ISR", err: kerr.NotEnoughReplicas, want: biz.PublishTemporary, isr: true},
		{name: "delivery timeout", err: kgo.ErrRecordTimeout, want: biz.PublishUncertain},
		{name: "request timeout", err: kerr.RequestTimedOut, want: biz.PublishUncertain},
		{name: "append below ISR", err: kerr.NotEnoughReplicasAfterAppend, want: biz.PublishUncertain, isr: true},
		{name: "context deadline", err: context.DeadlineExceeded, want: biz.PublishUncertain},
		{name: "message too large", err: kerr.MessageTooLarge, want: biz.PublishPermanent},
		{name: "unknown", err: errors.New("unknown publish failure"), want: biz.PublishPermanent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyPublishError(test.err); got != test.want {
				t.Errorf("classifyPublishError(%v) = %v, want %v", test.err, got, test.want)
			}
			if got := isISRFailure(test.err); got != test.isr {
				t.Errorf("isISRFailure(%v) = %t, want %t", test.err, got, test.isr)
			}
		})
	}
}
