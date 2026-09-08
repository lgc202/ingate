package telemetry

import "github.com/prometheus/client_golang/prometheus"

// NewTraceCollector 导出进程内 Trace 缓冲因容量不足而丢弃的 Span 数量。
func NewTraceCollector(tracing *Tracing) prometheus.Collector {
	return prometheus.NewCounterFunc(prometheus.CounterOpts{
		Namespace:   "ingate",
		Subsystem:   "telemetry",
		Name:        "spans_dropped_total",
		Help:        "Spans dropped before reaching the configured OTLP backend.",
		ConstLabels: prometheus.Labels{"reason": "queue_full"},
	}, func() float64 {
		return float64(tracing.Drops().QueueFull)
	})
}
