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
	"github.com/lgc202/ingate/internal/als/conf"
)

type readyPublisher struct{}

type readyQueue struct{}

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

func (readyQueue) Pending() (int64, int64) {
	return 0, 0
}

// TestReadyUsesCachedTopicStatus 验证就绪检查只根据缓存状态和 WAL 能力作出判断。
func TestReadyUsesCachedTopicStatus(t *testing.T) {
	tests := []struct {
		name        string
		topology    *biz.TopicTopology
		statusCode  int
		writeTarget string
	}{
		{
			name:        "compliant topic",
			topology:    &biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1},
			statusCode:  http.StatusOK,
			writeTarget: "kafka",
		},
		{
			name:        "topic check unavailable",
			statusCode:  http.StatusOK,
			writeTarget: "disk_queue",
		},
		{
			name:        "noncompliant topic",
			topology:    &biz.TopicTopology{},
			statusCode:  http.StatusServiceUnavailable,
			writeTarget: "none",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			topic := biz.NewTopicContract(biz.ReliabilityDevelopment)
			if test.topology != nil {
				topic.Update(*test.topology)
			}

			recorder := biz.NewRecorder(readyPublisher{}, topic, readyQueue{}, logger)
			handler := ready(&conf.Data_DiskQueue{MaxBytes: 1024}, recorder)
			response := httptest.NewRecorder()
			handler(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if response.Code != test.statusCode {
				t.Errorf("GET /readyz with %s status = %d, want %d", test.name, response.Code, test.statusCode)
			}

			var body struct {
				WriteTarget string `json:"write_target"`
			}
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode /readyz response: %v", err)
			}
			if body.WriteTarget != test.writeTarget {
				t.Errorf("GET /readyz with %s write_target = %q, want %q", test.name, body.WriteTarget, test.writeTarget)
			}
		})
	}
}
