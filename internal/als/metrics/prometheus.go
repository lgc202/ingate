// Package metrics 将 ALS 的可靠性状态和关键操作导出为 Prometheus 指标。
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lgc202/ingate/internal/als/biz"
)

const (
	metricNamespace = "ingate"
	metricSubsystem = "als"
)

var (
	validDesc         = newDesc("records_valid_total", "Request records that passed protocol validation.")
	kafkaAcceptedDesc = newDesc(
		"records_kafka_accepted_total",
		"Request records acknowledged by Kafka, including replayed duplicates.",
	)
	spooledDesc  = newDesc("records_spooled_total", "Request records appended to the disk queue.")
	replayedDesc = newDesc(
		"records_replayed_total",
		"Queued request records successfully published to Kafka, including duplicates after commit failures.",
	)
	committedDesc = newDesc(
		"records_committed_total",
		"Queued request records removed after successful Kafka publication.",
	)
	rejectedDesc = newDesc(
		"records_rejected_total",
		"Request records not fully acknowledged by Kafka and not appended to the disk queue.",
	)
	discardedDesc = newDesc(
		"records_discarded_total",
		"Malformed or unsupported access log records discarded at the protocol boundary.",
	)
	queueEntriesDesc      = newDesc("disk_queue_entries", "WAL entries currently waiting in the disk queue.")
	queueRecordsDesc      = newDesc("disk_queue_records", "Request records currently waiting in the disk queue.")
	queuePayloadBytesDesc = newDesc(
		"disk_queue_payload_bytes",
		"Protobuf payload bytes currently waiting in the disk queue.",
	)
	queueDiskBytesDesc     = newDesc("disk_queue_disk_bytes", "Physical bytes occupied by the disk queue directory.")
	queueCapacityBytesDesc = newDesc(
		"disk_queue_capacity_bytes",
		"Physical byte limit of the disk queue directory.",
	)
	queueUtilizationDesc = newDesc(
		"disk_queue_utilization_ratio",
		"Ratio of physical disk queue bytes to its configured capacity.",
	)
	queueOldestAgeDesc = newDesc(
		"disk_queue_oldest_entry_age_seconds",
		"Age of the oldest uncommitted WAL entry, or zero when the queue is empty.",
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
	replayPausedDesc = newDesc(
		"replay_paused",
		"Whether automatic disk queue replay stopped after a permanent publish failure.",
	)
	kafkaWritableDesc = newDesc("kafka_writable", "Whether the latest Kafka write operation succeeded.")

	_ prometheus.Collector = (*StatusCollector)(nil)
)

// StatusCollector 从 Recorder 的内存快照导出可靠投递和 WAL 状态。
type StatusCollector struct {
	recorder *biz.Recorder
}

// NewStatusCollector 创建 ALS 状态指标采集器。
func NewStatusCollector(recorder *biz.Recorder) *StatusCollector {
	return &StatusCollector{recorder: recorder}
}

// Describe 实现 Prometheus Collector。
func (c *StatusCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect 导出一次一致的 ALS 状态。
func (c *StatusCollector) Collect(ch chan<- prometheus.Metric) {
	status := c.recorder.Status()
	counters := c.recorder.Counters()
	queue := status.Queue

	ch <- prometheus.MustNewConstMetric(validDesc, prometheus.CounterValue, float64(counters.Valid))
	ch <- prometheus.MustNewConstMetric(kafkaAcceptedDesc, prometheus.CounterValue, float64(counters.KafkaAccepted))
	ch <- prometheus.MustNewConstMetric(spooledDesc, prometheus.CounterValue, float64(counters.Spooled))
	ch <- prometheus.MustNewConstMetric(replayedDesc, prometheus.CounterValue, float64(counters.Replayed))
	ch <- prometheus.MustNewConstMetric(committedDesc, prometheus.CounterValue, float64(counters.Committed))
	ch <- prometheus.MustNewConstMetric(rejectedDesc, prometheus.CounterValue, float64(counters.Rejected))
	ch <- prometheus.MustNewConstMetric(discardedDesc, prometheus.CounterValue, float64(counters.Discarded))

	ch <- prometheus.MustNewConstMetric(queueEntriesDesc, prometheus.GaugeValue, float64(queue.PendingEntries))
	ch <- prometheus.MustNewConstMetric(queueRecordsDesc, prometheus.GaugeValue, float64(queue.PendingRecords))
	ch <- prometheus.MustNewConstMetric(queuePayloadBytesDesc, prometheus.GaugeValue, float64(queue.PendingBytes))
	ch <- prometheus.MustNewConstMetric(queueDiskBytesDesc, prometheus.GaugeValue, float64(queue.DiskBytes))
	ch <- prometheus.MustNewConstMetric(queueCapacityBytesDesc, prometheus.GaugeValue, float64(queue.CapacityBytes))
	ch <- prometheus.MustNewConstMetric(queueUtilizationDesc, prometheus.GaugeValue, queueUtilization(queue))
	ch <- prometheus.MustNewConstMetric(queueOldestAgeDesc, prometheus.GaugeValue, oldestEntryAge(queue))
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
	ch <- prometheus.MustNewConstMetric(replayPausedDesc, prometheus.GaugeValue, boolValue(status.ReplayPaused))
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

func queueUtilization(status biz.QueueStatus) float64 {
	if status.CapacityBytes <= 0 {
		return 0
	}
	return float64(status.DiskBytes) / float64(status.CapacityBytes)
}

func oldestEntryAge(status biz.QueueStatus) float64 {
	if status.OldestEnqueuedAt.IsZero() {
		return 0
	}
	return max(time.Since(status.OldestEnqueuedAt).Seconds(), 0)
}

func boolValue(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
