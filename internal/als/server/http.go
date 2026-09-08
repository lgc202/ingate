package server

import (
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
	"github.com/lgc202/ingate/internal/pkg/telemetry"
)

// NewHTTPServer 创建健康检查、就绪检查和 Prometheus 指标服务。
func NewHTTPServer(
	serverConfig *conf.Server,
	queueConfig *conf.Data_DiskQueue,
	recorder *biz.Recorder,
	tracing *telemetry.Tracing,
) *kratoshttp.Server {
	httpConfig := serverConfig.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(queueConfig, recorder))
	server.Handle("/metrics", metricsHandler(recorder, tracing, queueConfig.GetMaxBytes()))
	return server
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

// ready 在 Kafka 可直写或磁盘队列可承接记录时报告就绪。
// 已确认的弱拓扑属于部署错误并返回不可用；Kafka 暂时不可达时则允许 WAL 维持服务。
func ready(
	queueConfig *conf.Data_DiskQueue,
	recorder *biz.Recorder,
) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		status := recorder.Status()
		if status.Topic.Checked && !status.Topic.Compliant {
			writeJSON(response, http.StatusServiceUnavailable, map[string]any{
				"status":              "unavailable",
				"write_target":        "none",
				"topic_contract":      "noncompliant",
				"replication_factor":  status.Topic.ReplicationFactor,
				"min_insync_replicas": status.Topic.MinInSyncReplicas,
				"pending_records":     status.PendingRecords,
				"pending_bytes":       status.PendingBytes,
			})
			return
		}
		queueFull := status.PendingBytes >= queueConfig.GetMaxBytes()
		canQueue := status.QueueWritable && !queueFull
		canWriteKafka := status.Topic.Compliant && !status.Spooling
		if !canWriteKafka && !canQueue {
			writeJSON(response, http.StatusServiceUnavailable, map[string]any{
				"status":          "unavailable",
				"write_target":    "none",
				"pending_records": status.PendingRecords,
				"pending_bytes":   status.PendingBytes,
			})
			return
		}
		target := "kafka"
		if !canWriteKafka {
			// Kafka 短暂故障不应立即摘除 ALS；只要磁盘队列仍可写，组件就能继续无损接收记录
			target = "disk_queue"
		}
		writeJSON(response, http.StatusOK, map[string]any{
			"status":          "ready",
			"write_target":    target,
			"pending_records": status.PendingRecords,
			"pending_bytes":   status.PendingBytes,
		})
	}
}

func metricsHandler(
	recorder *biz.Recorder,
	tracing *telemetry.Tracing,
	queueCapacity int64,
) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_accepted_total",
			Help:      "Request records accepted by Kafka or the disk queue.",
		}, func() float64 { return float64(recorder.Counters().Accepted) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_queued_total",
			Help:      "Request records written to the disk queue.",
		}, func() float64 { return float64(recorder.Counters().Queued) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_replayed_total",
			Help:      "Queued request records replayed to Kafka.",
		}, func() float64 { return float64(recorder.Counters().Replayed) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_rejected_total",
			Help:      "Request records rejected because Kafka and the disk queue were unavailable.",
		}, func() float64 { return float64(recorder.Counters().Rejected) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_discarded_total",
			Help:      "Malformed or unsupported access log records discarded at the protocol boundary.",
		}, func() float64 { return float64(recorder.Counters().Discarded) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "disk_queue_records",
			Help:      "Request records currently waiting in the disk queue.",
		}, func() float64 { return float64(recorder.Status().PendingRecords) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "disk_queue_bytes",
			Help:      "Protobuf payload bytes currently waiting in the disk queue.",
		}, func() float64 { return float64(recorder.Status().PendingBytes) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "disk_queue_capacity_bytes",
			Help:      "Configured logical capacity of the disk queue in protobuf payload bytes.",
		}, func() float64 { return float64(queueCapacity) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "spooling",
			Help:      "Whether new request records are currently being written to the disk queue.",
		}, func() float64 { return boolMetric(recorder.Status().Spooling) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "kafka_writable",
			Help:      "Whether the latest Kafka write operation succeeded.",
		}, func() float64 { return boolMetric(recorder.Status().KafkaWritable) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace:   "ingate",
			Subsystem:   "telemetry",
			Name:        "spans_dropped_total",
			Help:        "Spans dropped before reaching the configured OTLP backend.",
			ConstLabels: prometheus.Labels{"reason": "queue_full"},
		}, func() float64 { return float64(tracing.Drops().QueueFull) }),
	)

	// 使用独立 Registry 只注册 Go、进程和 ALS 可靠性指标，避免依赖库隐式污染指标空间
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func boolMetric(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func writeJSON(response http.ResponseWriter, statusCode int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(statusCode)
	// 响应头已经发出，客户端断开导致的编码错误无法再转换为另一份 HTTP 响应。
	_ = json.NewEncoder(response).Encode(value)
}
