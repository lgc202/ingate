package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lgc202/ingate/internal/als/biz"
)

func newMetricsHandler(recorder *biz.Recorder, queueCapacity int64) http.Handler {
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
			Name:      "kafka_reachable",
			Help:      "Whether the latest Kafka delivery operation succeeded.",
		}, func() float64 { return boolMetric(recorder.DeliveryStatus().KafkaReachable) }),
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
