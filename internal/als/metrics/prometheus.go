// Package metrics 将 ALS 的可靠性状态导出为 Prometheus 指标。
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lgc202/ingate/internal/als/biz"
)

const (
	metricNamespace = "ingate"
	metricSubsystem = "als"
)

var (
	acceptedDesc = newDesc(
		"records_accepted_total",
		"Request records accepted by Kafka or the disk queue.",
	)
	queuedDesc = newDesc(
		"records_queued_total",
		"Request records written to the disk queue.",
	)
	replayedDesc = newDesc(
		"records_replayed_total",
		"Queued request records replayed to Kafka.",
	)
	rejectedDesc = newDesc(
		"records_rejected_total",
		"Request records rejected because Kafka and the disk queue were unavailable.",
	)
	discardedDesc = newDesc(
		"records_discarded_total",
		"Malformed or unsupported access log records discarded at the protocol boundary.",
	)
	queueRecordsDesc = newDesc(
		"disk_queue_records",
		"Request records currently waiting in the disk queue.",
	)
	queuePayloadBytesDesc = newDesc(
		"disk_queue_payload_bytes",
		"Protobuf payload bytes currently waiting in the disk queue.",
	)
	queueDiskBytesDesc = newDesc(
		"disk_queue_disk_bytes",
		"Physical bytes occupied by the disk queue directory.",
	)
	queueCapacityBytesDesc = newDesc(
		"disk_queue_capacity_bytes",
		"Physical byte limit of the disk queue directory.",
	)
	queueFreeBytesDesc = newDesc(
		"disk_queue_filesystem_free_bytes",
		"Bytes available to ALS on the disk queue filesystem.",
	)
	queueMinFreeBytesDesc = newDesc(
		"disk_queue_minimum_free_bytes",
		"Filesystem safety reserve in addition to WAL recovery headroom.",
	)
	queueWritableDesc = newDesc(
		"disk_queue_writable",
		"Whether the disk queue can reliably append a new batch.",
	)
	queueStateDesc = newDesc(
		"disk_queue_state",
		"Current disk queue capacity state as a one-hot gauge.",
		"state",
	)
	spoolingDesc = newDesc(
		"spooling",
		"Whether new request records are currently being written to the disk queue.",
	)
	kafkaWritableDesc = newDesc(
		"kafka_writable",
		"Whether the latest Kafka write operation succeeded.",
	)
)

var _ prometheus.Collector = (*collector)(nil)

// collector 每次采集只读取一次 Recorder 状态，避免每个 WAL 指标各自扫描目录。
type collector struct {
	recorder *biz.Recorder
}

// NewCollector 创建 ALS 可靠性指标采集器。
func NewCollector(recorder *biz.Recorder) prometheus.Collector {
	return &collector{recorder: recorder}
}

// Describe 实现 Prometheus Collector。
func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect 导出一次一致的 ALS 状态。
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	status := c.recorder.Status()
	counters := c.recorder.Counters()
	queue := status.Queue

	ch <- prometheus.MustNewConstMetric(acceptedDesc, prometheus.CounterValue, float64(counters.Accepted))
	ch <- prometheus.MustNewConstMetric(queuedDesc, prometheus.CounterValue, float64(counters.Queued))
	ch <- prometheus.MustNewConstMetric(replayedDesc, prometheus.CounterValue, float64(counters.Replayed))
	ch <- prometheus.MustNewConstMetric(rejectedDesc, prometheus.CounterValue, float64(counters.Rejected))
	ch <- prometheus.MustNewConstMetric(discardedDesc, prometheus.CounterValue, float64(counters.Discarded))

	ch <- prometheus.MustNewConstMetric(queueRecordsDesc, prometheus.GaugeValue, float64(queue.PendingRecords))
	ch <- prometheus.MustNewConstMetric(queuePayloadBytesDesc, prometheus.GaugeValue, float64(queue.PendingBytes))
	ch <- prometheus.MustNewConstMetric(queueDiskBytesDesc, prometheus.GaugeValue, float64(queue.DiskBytes))
	ch <- prometheus.MustNewConstMetric(queueCapacityBytesDesc, prometheus.GaugeValue, float64(queue.CapacityBytes))
	ch <- prometheus.MustNewConstMetric(queueFreeBytesDesc, prometheus.GaugeValue, float64(queue.FreeBytes))
	ch <- prometheus.MustNewConstMetric(queueMinFreeBytesDesc, prometheus.GaugeValue, float64(queue.MinFreeBytes))
	ch <- prometheus.MustNewConstMetric(queueWritableDesc, prometheus.GaugeValue, boolValue(queue.Writable))

	for _, state := range [...]biz.QueueState{
		biz.QueueHealthy,
		biz.QueueWarning,
		biz.QueueCritical,
		biz.QueueBlocked,
	} {
		ch <- prometheus.MustNewConstMetric(
			queueStateDesc,
			prometheus.GaugeValue,
			boolValue(queue.State == state),
			state.String(),
		)
	}

	ch <- prometheus.MustNewConstMetric(spoolingDesc, prometheus.GaugeValue, boolValue(status.Spooling))
	ch <- prometheus.MustNewConstMetric(kafkaWritableDesc, prometheus.GaugeValue, boolValue(status.KafkaWritable))
}

func newDesc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(metricNamespace, metricSubsystem, name),
		help,
		labels,
		nil,
	)
}

func boolValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
