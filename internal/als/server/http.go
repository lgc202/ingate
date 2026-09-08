package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	alsmetrics "github.com/lgc202/ingate/internal/als/metrics"
	"github.com/lgc202/ingate/internal/pkg/prometheus"
	"github.com/lgc202/ingate/internal/pkg/telemetry"
)

const (
	reasonTopicNoncompliant = "topic_noncompliant"
	reasonReplayPaused      = "replay_paused"
	reasonWALUnavailable    = "wal_unavailable"
)

type readinessResponse struct {
	Status            string `json:"status"`
	Reason            string `json:"reason,omitempty"`
	WriteTarget       string `json:"write_target"`
	TopicContract     string `json:"topic_contract,omitempty"`
	ReplicationFactor int    `json:"replication_factor,omitempty"`
	MinInSyncReplicas int    `json:"min_insync_replicas,omitempty"`
	QueueState        string `json:"queue_state"`
	QueueWritable     bool   `json:"queue_writable"`
	PendingRecords    int64  `json:"pending_records"`
	PendingBytes      int64  `json:"pending_bytes"`
}

// NewHTTPServer 创建健康检查、就绪检查和 Prometheus 指标服务。
func NewHTTPServer(
	serverConfig *conf.Server,
	recorder *biz.Recorder,
	events *alsmetrics.EventCollector,
	tracing *telemetry.Tracing,
) *kratoshttp.Server {
	httpConfig := serverConfig.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/livez", health)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(recorder))
	server.Handle("/metrics", prometheus.NewHandler(
		alsmetrics.NewStatusCollector(recorder),
		events,
		telemetry.NewTraceCollector(tracing),
	))
	return server
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

// ready 在 WAL 具备故障兜底能力，且 Kafka 可直写或 WAL 可承接当前记录时报告就绪。
// 即使 Kafka 当前正常，容量阻塞也会返回不可用，避免把失去可靠降级能力的实例继续留在服务发现中。
func ready(recorder *biz.Recorder) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		status := recorder.Status()
		body := readinessResponse{
			Status:         "ready",
			WriteTarget:    "kafka",
			QueueState:     status.Queue.State.String(),
			QueueWritable:  status.Queue.Writable,
			PendingRecords: status.Queue.PendingRecords,
			PendingBytes:   status.Queue.PendingBytes,
		}

		if status.Topic.Checked && !status.Topic.Compliant {
			body.Status = "unavailable"
			body.Reason = reasonTopicNoncompliant
			body.WriteTarget = "none"
			body.TopicContract = "noncompliant"
			body.ReplicationFactor = status.Topic.ReplicationFactor
			body.MinInSyncReplicas = status.Topic.MinInSyncReplicas
			writeJSON(response, http.StatusServiceUnavailable, body)
			return
		}
		if status.ReplayPaused {
			body.Status = "unavailable"
			body.Reason = reasonReplayPaused
			body.WriteTarget = "none"
			writeJSON(response, http.StatusServiceUnavailable, body)
			return
		}
		if !status.Queue.Writable {
			body.Status = "unavailable"
			body.Reason = reasonWALUnavailable
			body.WriteTarget = "none"
			writeJSON(response, http.StatusServiceUnavailable, body)
			return
		}
		if !status.Topic.Compliant || status.Spooling {
			// Kafka 短暂故障不应立即摘除 ALS；只要磁盘队列仍可写，组件就能继续无损接收记录
			body.WriteTarget = "disk_queue"
		}
		writeJSON(response, http.StatusOK, body)
	}
}

func writeJSON(response http.ResponseWriter, statusCode int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	// 响应头已经发出，客户端断开导致的编码错误无法再转换为另一份 HTTP 响应。
	_ = json.NewEncoder(response).Encode(value)
}
