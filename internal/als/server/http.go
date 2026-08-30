package server

import (
	"context"
	"encoding/json"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

// NewHTTPServer 创建健康检查、就绪检查和 Prometheus 指标服务。
func NewHTTPServer(
	serverConfig *conf.Server,
	kafkaConfig *conf.Data_Kafka,
	queueConfig *conf.Data_DiskQueue,
	recorder *biz.Recorder,
) *kratoshttp.Server {
	httpConfig := serverConfig.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	server.HandleFunc("/healthz", health)
	server.HandleFunc("/readyz", ready(kafkaConfig, queueConfig, recorder))
	server.Handle("/metrics", metricsHandler(recorder, queueConfig.GetMaxBytes()))
	return server
}

func health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func ready(
	kafkaConfig *conf.Data_Kafka,
	queueConfig *conf.Data_DiskQueue,
	recorder *biz.Recorder,
) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), kafkaConfig.GetReadinessTimeout().AsDuration())
		defer cancel()
		deliveryStatus := recorder.DeliveryStatus()
		kafkaErr := recorder.CheckKafka(ctx)
		queueFull := deliveryStatus.PendingBytes >= queueConfig.GetMaxBytes()
		canQueue := deliveryStatus.QueueWritable && !queueFull
		canWriteKafka := kafkaErr == nil && !deliveryStatus.Spooling
		if !canWriteKafka && !canQueue {
			writeJSON(response, http.StatusServiceUnavailable, map[string]any{
				"status":          "unavailable",
				"delivery":        "none",
				"pending_records": deliveryStatus.PendingRecords,
				"pending_bytes":   deliveryStatus.PendingBytes,
			})
			return
		}
		delivery := "kafka"
		if !canWriteKafka {
			// Kafka 短暂故障不应立即摘除 ALS；只要磁盘队列仍可写，组件就能继续无损接收记录
			delivery = "disk_queue"
		}
		writeJSON(response, http.StatusOK, map[string]any{
			"status":          "ready",
			"delivery":        delivery,
			"pending_records": deliveryStatus.PendingRecords,
			"pending_bytes":   deliveryStatus.PendingBytes,
		})
	}
}

func metricsHandler(recorder *biz.Recorder, queueCapacity int64) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_accepted_total",
			Help:      "Request records accepted by Kafka or the disk queue.",
		}, func() float64 { return float64(recorder.DeliveryCounters().Accepted) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_queued_total",
			Help:      "Request records written to the disk queue.",
		}, func() float64 { return float64(recorder.DeliveryCounters().Queued) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_replayed_total",
			Help:      "Queued request records replayed to Kafka.",
		}, func() float64 { return float64(recorder.DeliveryCounters().Replayed) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_rejected_total",
			Help:      "Request records rejected because Kafka and the disk queue were unavailable.",
		}, func() float64 { return float64(recorder.DeliveryCounters().Rejected) }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "records_discarded_total",
			Help:      "Malformed or unsupported access log records discarded at the protocol boundary.",
		}, func() float64 { return float64(recorder.DeliveryCounters().Discarded) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "disk_queue_records",
			Help:      "Request records currently waiting in the disk queue.",
		}, func() float64 { return float64(recorder.DeliveryStatus().PendingRecords) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "disk_queue_bytes",
			Help:      "Protobuf payload bytes currently waiting in the disk queue.",
		}, func() float64 { return float64(recorder.DeliveryStatus().PendingBytes) }),
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
		}, func() float64 { return boolMetric(recorder.DeliveryStatus().Spooling) }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "ingate",
			Subsystem: "als",
			Name:      "kafka_writable",
			Help:      "Whether the latest Kafka delivery operation succeeded.",
		}, func() float64 { return boolMetric(recorder.DeliveryStatus().KafkaWritable) }),
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
	_ = json.NewEncoder(response).Encode(value)
}
