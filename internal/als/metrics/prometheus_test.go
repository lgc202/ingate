package metrics

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

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

// TestCollectorReportsQueueState 验证指标同时暴露容量数值和一位有效的状态标签。
func TestCollectorReportsQueueState(t *testing.T) {
	queue := testQueue{status: biz.QueueStatus{
		State:          biz.QueueCritical,
		Writable:       true,
		PendingRecords: 3,
		PendingBytes:   512,
		DiskBytes:      800,
		CapacityBytes:  1_000,
		FreeBytes:      400,
		MinFreeBytes:   100,
	}}
	recorder := biz.NewRecorder(
		testPublisher{},
		biz.NewTopicContract(biz.ReliabilityDevelopment),
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewCollector(recorder))
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Registry.Gather() error = %v, want nil", err)
	}

	var critical float64
	for _, family := range families {
		if family.GetName() != "ingate_als_disk_queue_state" {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metric.GetLabel()[0].GetValue() == biz.QueueCritical.String() {
				critical = metric.GetGauge().GetValue()
			}
		}
	}
	if critical != 1 {
		t.Errorf("ingate_als_disk_queue_state{state=\"critical\"} = %v, want 1", critical)
	}
}
