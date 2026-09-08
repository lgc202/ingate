package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
)

type readyPublisher struct {
	result biz.PublishResult
}

type readyQueue struct {
	status biz.QueueStatus
}

func (p readyPublisher) Publish(_ context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	if p.result.Err != nil {
		return p.result
	}
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

// TestHealthReportsProcessLiveness 验证存活端点无需任何 Kafka 或 WAL 依赖即可响应。
func TestHealthReportsProcessLiveness(t *testing.T) {
	response := httptest.NewRecorder()
	health(response, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if response.Code != http.StatusOK {
		t.Errorf("GET /livez status = %d, want %d", response.Code, http.StatusOK)
	}
}

// TestReadyUsesCachedTopicStatus 验证就绪检查只根据缓存状态和 WAL 能力作出判断。
func TestReadyUsesCachedTopicStatus(t *testing.T) {
	tests := []struct {
		name          string
		topology      *biz.TopicTopology
		queue         biz.QueueStatus
		statusCode    int
		reason        string
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
			reason:        reasonTopicNoncompliant,
			writeTarget:   "none",
			queueState:    "healthy",
			queueWritable: true,
		},
		{
			name:        "blocked queue",
			topology:    &biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1},
			queue:       biz.QueueStatus{State: biz.QueueBlocked},
			statusCode:  http.StatusServiceUnavailable,
			reason:      reasonWALUnavailable,
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
				Reason        string `json:"reason"`
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
			if body.Reason != test.reason {
				t.Errorf("GET /readyz with %s reason = %q, want %q", test.name, body.Reason, test.reason)
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

// TestReadyReportsPausedReplay 验证永久发布错误使用稳定原因码摘除实例。
func TestReadyReportsPausedReplay(t *testing.T) {
	queue := &replayerQueue{records: []*alsv1.RequestRecord{{Id: "record-1"}}}
	topic := biz.NewTopicContract(biz.ReliabilityDevelopment)
	topic.Update(biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := biz.NewRecorder(
		readyPublisher{result: biz.PublishResult{
			Failed: 1,
			Class:  biz.PublishPermanent,
			Err:    errors.New("record is invalid"),
		}},
		topic,
		queue,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	if result, err := recorder.ReplayBatch(t.Context(), 1); result != biz.ReplayPaused || err == nil {
		t.Fatalf("Recorder.ReplayBatch() = (%v, %v), want (%v, non-nil)", result, err, biz.ReplayPaused)
	}

	response := httptest.NewRecorder()
	ready(recorder)(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /readyz status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode /readyz response: %v", err)
	}
	if body.Reason != reasonReplayPaused {
		t.Errorf("GET /readyz reason = %q, want %q", body.Reason, reasonReplayPaused)
	}
}

func writableQueue(state biz.QueueState) biz.QueueStatus {
	return biz.QueueStatus{State: state, Writable: true}
}
