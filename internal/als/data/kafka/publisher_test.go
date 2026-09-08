package kafka

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
)

// TestPublishAggregatesRecordFailures 验证同步发布保留每条记录的最终结果。
func TestPublishAggregatesRecordFailures(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans))
	t.Cleanup(func() { _ = provider.Shutdown(t.Context()) })

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
		tracer: provider.Tracer("test"),
	}
	ctx, parent := provider.Tracer("test").Start(t.Context(), "als.receive_batch")
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	result := publisher.Publish(ctx, []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}})
	parent.End()
	if result.Confirmed != 0 || result.Failed != 2 {
		t.Errorf("Client.Publish() counts = (%d confirmed, %d failed), want (0, 2)", result.Confirmed, result.Failed)
	}
	if result.Class != biz.PublishUncertain || !errors.Is(result.Err, context.Canceled) {
		t.Errorf("Client.Publish() failure = (%v, %v), want uncertain context cancellation", result.Class, result.Err)
	}
	assertKafkaPublishSpan(t, spans.Ended(), parent.SpanContext())
}

// TestNewRecordsPropagatesTraceContext 验证 Kafka Header 使用标准 W3C Trace Context，且不混用 Request ID。
func TestNewRecordsPropagatesTraceContext(t *testing.T) {
	traceState, err := oteltrace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatalf("trace.ParseTraceState() error = %v, want nil", err)
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    oteltrace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     oteltrace.SpanID{17, 18, 19, 20, 21, 22, 23, 24},
		TraceFlags: oteltrace.FlagsSampled,
		TraceState: traceState,
	})
	ctx := oteltrace.ContextWithSpanContext(t.Context(), spanContext)
	records, err := newRecords(ctx, []*alsv1.RequestRecord{{
		Id:        "record-1",
		RequestId: "request-id-must-not-become-trace-id",
	}})
	if err != nil {
		t.Fatalf("newRecords() error = %v, want nil", err)
	}

	wantTraceparent := "00-0102030405060708090a0b0c0d0e0f10-1112131415161718-01"
	if got := recordHeader(records[0], "traceparent"); got != wantTraceparent {
		t.Errorf("traceparent header = %q, want %q", got, wantTraceparent)
	}
	if got := recordHeader(records[0], "tracestate"); got != traceState.String() {
		t.Errorf("tracestate header = %q, want %q", got, traceState.String())
	}
	if got := recordHeader(records[0], "traceparent"); strings.Contains(got, "request-id") {
		t.Errorf("traceparent header contains Request ID: %q", got)
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

func assertKafkaPublishSpan(
	t *testing.T,
	spans []sdktrace.ReadOnlySpan,
	parent oteltrace.SpanContext,
) {
	t.Helper()
	for _, span := range spans {
		if span.Name() != "als.kafka.publish" {
			continue
		}
		if span.SpanKind() != oteltrace.SpanKindProducer {
			t.Errorf("Kafka span kind = %v, want producer", span.SpanKind())
		}
		if span.Parent().SpanID() != parent.SpanID() {
			t.Errorf("Kafka span parent = %s, want %s", span.Parent().SpanID(), parent.SpanID())
		}
		for _, attribute := range span.Attributes() {
			if strings.Contains(attribute.Value.String(), "record-") {
				t.Errorf("Kafka span attribute %q contains request data", attribute.Key)
			}
		}
		return
	}
	t.Error("als.kafka.publish span not found")
}

func recordHeader(record *kgo.Record, key string) string {
	for _, header := range record.Headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}
