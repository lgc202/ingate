package metrics

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
)

type testPublisher struct{}

type testQueue struct {
	status biz.QueueStatus
}

func (testPublisher) Publish(_ context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	return biz.PublishResult{Confirmed: len(records)}
}

func (testQueue) Write(context.Context, []*alsv1.RequestRecord) error {
	return nil
}

func (testQueue) Read(context.Context, int) (biz.QueuedBatch, error) {
	return biz.QueuedBatch{}, biz.ErrQueueEmpty
}

func (testQueue) Commit(context.Context, biz.QueuedBatch) error {
	return nil
}

func (q testQueue) Pending() (int64, int64) {
	return q.status.PendingRecords, q.status.PendingBytes
}

func (q testQueue) Status() biz.QueueStatus {
	return q.status
}

// TestStatusCollectorReportsQueueState 验证指标同时暴露容量数值和一位有效的状态标签。
func TestStatusCollectorReportsQueueState(t *testing.T) {
	queue := testQueue{status: biz.QueueStatus{
		State:            biz.QueueCritical,
		Writable:         true,
		PendingEntries:   2,
		PendingRecords:   3,
		PendingBytes:     512,
		OldestEnqueuedAt: time.Now().Add(-time.Minute),
		DiskBytes:        800,
		CapacityBytes:    1_000,
		FreeBytes:        400,
		MinFreeBytes:     100,
	}}
	recorder := biz.NewRecorder(
		testPublisher{},
		biz.NewTopicContract(biz.ReliabilityDevelopment),
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if err := recorder.Write(t.Context(), []*alsv1.RequestRecord{{Id: "record-1"}}); err != nil {
		t.Fatalf("Recorder.Write() error = %v, want nil", err)
	}
	recorder.Discard(1)

	err := testutil.CollectAndCompare(
		NewStatusCollector(recorder),
		strings.NewReader(`# HELP ingate_als_disk_queue_entries WAL entries currently waiting in the disk queue.
# TYPE ingate_als_disk_queue_entries gauge
ingate_als_disk_queue_entries 2
# HELP ingate_als_disk_queue_state Current disk queue capacity state as a one-hot gauge.
# TYPE ingate_als_disk_queue_state gauge
ingate_als_disk_queue_state{state="blocked"} 0
ingate_als_disk_queue_state{state="critical"} 1
ingate_als_disk_queue_state{state="healthy"} 0
ingate_als_disk_queue_state{state="warning"} 0
# HELP ingate_als_disk_queue_utilization_ratio Ratio of physical disk queue bytes to its configured capacity.
# TYPE ingate_als_disk_queue_utilization_ratio gauge
ingate_als_disk_queue_utilization_ratio 0.8
# HELP ingate_als_records_discarded_total Malformed or unsupported access log records discarded at the protocol boundary.
# TYPE ingate_als_records_discarded_total counter
ingate_als_records_discarded_total 1
# HELP ingate_als_records_spooled_total Request records appended to the disk queue.
# TYPE ingate_als_records_spooled_total counter
ingate_als_records_spooled_total 1
# HELP ingate_als_records_valid_total Request records that passed protocol validation.
# TYPE ingate_als_records_valid_total counter
ingate_als_records_valid_total 1
`),
		"ingate_als_disk_queue_entries",
		"ingate_als_disk_queue_state",
		"ingate_als_disk_queue_utilization_ratio",
		"ingate_als_records_discarded_total",
		"ingate_als_records_spooled_total",
		"ingate_als_records_valid_total",
	)
	if err != nil {
		t.Fatalf("CollectAndCompare() error = %v, want nil", err)
	}
}

// TestEventCollectorReportsOperations 验证事件指标只使用有限标签并保留实际计数。
func TestEventCollectorReportsOperations(t *testing.T) {
	collector := NewEventCollector()
	collector.StreamStarted()
	collector.ObserveBatch(3, 20*time.Millisecond)
	collector.ObserveKafkaPublish(10*time.Millisecond, biz.PublishTemporary)
	collector.AddKafkaISRFailures(2)
	collector.SetReplayBackoff(4 * time.Second)

	err := testutil.CollectAndCompare(
		collector,
		strings.NewReader(`# HELP ingate_als_batches_received_total Access log batches received from Envoy.
# TYPE ingate_als_batches_received_total counter
ingate_als_batches_received_total 1
# HELP ingate_als_kafka_isr_failures_total Kafka record failures caused by insufficient in-sync replicas.
# TYPE ingate_als_kafka_isr_failures_total counter
ingate_als_kafka_isr_failures_total 2
# HELP ingate_als_kafka_publish_failures_total Kafka publish failures by stable delivery classification.
# TYPE ingate_als_kafka_publish_failures_total counter
ingate_als_kafka_publish_failures_total{class="temporary"} 1
# HELP ingate_als_records_received_total Request records received at the ALS protocol boundary.
# TYPE ingate_als_records_received_total counter
ingate_als_records_received_total 3
# HELP ingate_als_replay_backoff_seconds Current delay before retrying disk queue replay, or zero when not backing off.
# TYPE ingate_als_replay_backoff_seconds gauge
ingate_als_replay_backoff_seconds 4
# HELP ingate_als_streams_active Current Envoy ALS streams.
# TYPE ingate_als_streams_active gauge
ingate_als_streams_active 1
`),
		"ingate_als_batches_received_total",
		"ingate_als_kafka_isr_failures_total",
		"ingate_als_kafka_publish_failures_total",
		"ingate_als_records_received_total",
		"ingate_als_replay_backoff_seconds",
		"ingate_als_streams_active",
	)
	if err != nil {
		t.Fatalf("CollectAndCompare() error = %v, want nil", err)
	}

	collector.StreamFinished()
	if got := testutil.ToFloat64(collector.streams); got != 0 {
		t.Errorf("streams_active = %v, want 0", got)
	}
}
