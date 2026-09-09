package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lgc202/ingate/internal/als/biz"
)

var _ prometheus.Collector = (*EventCollector)(nil)

// EventCollector 记录无法从状态快照还原的 ALS 操作指标。
type EventCollector struct {
	activeStreams        prometheus.Gauge
	receivedBatches      prometheus.Counter
	receivedRecords      prometheus.Counter
	batchRecords         prometheus.Histogram
	batchDuration        prometheus.Histogram
	kafkaPublishDuration prometheus.Histogram
	kafkaPublishFailures *prometheus.CounterVec
	kafkaISRFailures     prometheus.Counter
	replayBackoff        prometheus.Gauge
	collectors           []prometheus.Collector
}

// NewEventCollector 创建 ALS 操作指标采集器。
func NewEventCollector() *EventCollector {
	collector := &EventCollector{
		activeStreams: newGauge("streams_active", "Current Envoy ALS streams."),
		receivedBatches: newCounter(
			"batches_received_total",
			"Access log batches received from Envoy.",
		),
		receivedRecords: newCounter(
			"records_received_total",
			"Request records received at the ALS protocol boundary.",
		),
		batchRecords: newHistogram(
			"batch_records",
			"Request records contained in each Envoy access log batch.",
			prometheus.ExponentialBuckets(1, 2, 11),
		),
		batchDuration: newHistogram(
			"batch_processing_seconds",
			"Time spent validating, converting, and durably accepting an access log batch.",
			nil,
		),
		kafkaPublishDuration: newHistogram(
			"kafka_publish_seconds",
			"Time spent waiting for a synchronous Kafka publish result.",
			nil,
		),
		kafkaPublishFailures: prometheus.NewCounterVec(
			counterOpts("kafka_publish_failures_total", "Kafka publish failures by stable publish classification."),
			[]string{"class"},
		),
		kafkaISRFailures: newCounter(
			"kafka_isr_failures_total",
			"Kafka record failures caused by insufficient in-sync replicas.",
		),
		replayBackoff: newGauge(
			"replay_backoff_seconds",
			"Current delay before retrying disk queue replay, or zero when not backing off.",
		),
	}
	collector.collectors = []prometheus.Collector{
		collector.activeStreams,
		collector.receivedBatches,
		collector.receivedRecords,
		collector.batchRecords,
		collector.batchDuration,
		collector.kafkaPublishDuration,
		collector.kafkaPublishFailures,
		collector.kafkaISRFailures,
		collector.replayBackoff,
	}
	return collector
}

// Describe 实现 Prometheus Collector。
func (c *EventCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, collector := range c.collectors {
		collector.Describe(ch)
	}
}

// Collect 实现 Prometheus Collector。
func (c *EventCollector) Collect(ch chan<- prometheus.Metric) {
	for _, collector := range c.collectors {
		collector.Collect(ch)
	}
}

// StreamStarted 记录一条已进入协议处理的 ALS 流。
func (c *EventCollector) StreamStarted() {
	c.activeStreams.Inc()
}

// StreamFinished 记录一条已退出协议处理的 ALS 流。
func (c *EventCollector) StreamFinished() {
	c.activeStreams.Dec()
}

// ObserveBatch 记录一个 Envoy 批次的规模和端到端处理时间。
func (c *EventCollector) ObserveBatch(records int, elapsed time.Duration) {
	c.receivedBatches.Inc()
	c.receivedRecords.Add(float64(records))
	c.batchRecords.Observe(float64(records))
	c.batchDuration.Observe(elapsed.Seconds())
}

// ObserveKafkaPublish 记录一次同步 Kafka 发布的耗时和失败类别。
func (c *EventCollector) ObserveKafkaPublish(elapsed time.Duration, class biz.PublishClass) {
	c.kafkaPublishDuration.Observe(elapsed.Seconds())
	if class != 0 {
		c.kafkaPublishFailures.WithLabelValues(class.String()).Inc()
	}
}

// AddKafkaISRFailures 累加因同步副本不足而失败的 Kafka 记录数。
func (c *EventCollector) AddKafkaISRFailures(count int) {
	if count > 0 {
		c.kafkaISRFailures.Add(float64(count))
	}
}

// SetReplayBackoff 设置当前回放重试等待时间；未退避时传入零值。
func (c *EventCollector) SetReplayBackoff(delay time.Duration) {
	c.replayBackoff.Set(delay.Seconds())
}

func newCounter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(counterOpts(name, help))
}

func counterOpts(name, help string) prometheus.CounterOpts {
	return prometheus.CounterOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      name,
		Help:      help,
	}
}

func newGauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      name,
		Help:      help,
	})
}

func newHistogram(name, help string, buckets []float64) prometheus.Histogram {
	return prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricNamespace,
		Subsystem: metricSubsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	})
}
