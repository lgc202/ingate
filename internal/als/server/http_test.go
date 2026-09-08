package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
)

type readyPublisher struct{}

type readyQueue struct {
	status biz.QueueStatus
}

func (readyPublisher) Publish(_ context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	return biz.PublishResult{Confirmed: len(records)}
}

func (readyQueue) Write(context.Context, []*alsv1.RequestRecord) error {
	return nil
}

func (readyQueue) Read(context.Context, int) (biz.QueuedBatch, error) {
	return biz.QueuedBatch{}, biz.ErrQueueEmpty
}

func (readyQueue) Commit(context.Context, biz.QueuedBatch) error {
	return nil
}

func (q readyQueue) Pending() (int64, int64) {
	return q.status.PendingRecords, q.status.PendingBytes
}

func (q readyQueue) Status() biz.QueueStatus {
	return q.status
}

// TestReadyUsesCachedTopicStatus 验证就绪检查只根据缓存状态和 WAL 能力作出判断。
func TestReadyUsesCachedTopicStatus(t *testing.T) {
	tests := []struct {
		name          string
		topology      *biz.TopicTopology
		queue         biz.QueueStatus
		statusCode    int
		writeTarget   string
		queueState    string
		queueWritable bool
	}{
		{
			name:          "compliant topic",
			topology:      &biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1},
			queue:         writableQueue(biz.QueueHealthy),
			statusCode:    http.StatusOK,
			writeTarget:   "kafka",
			queueState:    "healthy",
			queueWritable: true,
		},
		{
			name:          "topic check unavailable",
			queue:         writableQueue(biz.QueueWarning),
			statusCode:    http.StatusOK,
			writeTarget:   "disk_queue",
			queueState:    "warning",
			queueWritable: true,
		},
		{
			name:          "noncompliant topic",
			topology:      &biz.TopicTopology{},
			queue:         writableQueue(biz.QueueHealthy),
			statusCode:    http.StatusServiceUnavailable,
			writeTarget:   "none",
			queueState:    "healthy",
			queueWritable: true,
		},
		{
			name:        "blocked queue",
			topology:    &biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1},
			queue:       biz.QueueStatus{State: biz.QueueBlocked},
			statusCode:  http.StatusServiceUnavailable,
			writeTarget: "none",
			queueState:  "blocked",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			topic := biz.NewTopicContract(biz.ReliabilityDevelopment)
			if test.topology != nil {
				topic.Update(*test.topology)
			}

			recorder := biz.NewRecorder(readyPublisher{}, topic, readyQueue{status: test.queue}, logger)
			handler := ready(recorder)
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if response.Code != test.statusCode {
				t.Errorf("GET /readyz with %s status = %d, want %d", test.name, response.Code, test.statusCode)
			}

			var body struct {
				WriteTarget   string `json:"write_target"`
				QueueState    string `json:"queue_state"`
				QueueWritable bool   `json:"queue_writable"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode /readyz response: %v", err)
			}
			if body.WriteTarget != test.writeTarget {
				t.Errorf("GET /readyz with %s write_target = %q, want %q", test.name, body.WriteTarget, test.writeTarget)
			}
			if body.QueueState != test.queueState {
				t.Errorf("GET /readyz with %s queue_state = %q, want %q", test.name, body.QueueState, test.queueState)
			}
			if body.QueueWritable != test.queueWritable {
				t.Errorf("GET /readyz with %s queue_writable = %t, want %t", test.name, body.QueueWritable, test.queueWritable)
			}
		})
	}
}

func writableQueue(state biz.QueueState) biz.QueueStatus {
	return biz.QueueStatus{State: state, Writable: true}
}
