package biz

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

type stubPublisher struct {
	result PublishResult
	calls  int
}

type stubQueue struct {
	written  int
	writeErr error
}

type replayQueue struct {
	batch     QueuedBatch
	committed bool
}

func (p *stubPublisher) Publish(context.Context, []*alsv1.RequestRecord) PublishResult {
	p.calls++
	return p.result
}

func (q *stubQueue) Write(_ context.Context, records []*alsv1.RequestRecord) error {
	if q.writeErr != nil {
		return q.writeErr
	}
	q.written += len(records)
	return nil
}

func (q *stubQueue) Read(context.Context, int) (QueuedBatch, error) {
	return QueuedBatch{}, ErrQueueEmpty
}

func (q *stubQueue) Commit(context.Context, QueuedBatch) error {
	return nil
}

func (q *stubQueue) Pending() (int64, int64) {
	return int64(q.written), 0
}

func (q *replayQueue) Write(context.Context, []*alsv1.RequestRecord) error {
	return nil
}

func (q *replayQueue) Read(context.Context, int) (QueuedBatch, error) {
	if q.committed {
		return QueuedBatch{}, ErrQueueEmpty
	}
	return q.batch, nil
}

func (q *replayQueue) Commit(_ context.Context, batch QueuedBatch) error {
	if batch.LastSequence != q.batch.LastSequence {
		return errors.New("unexpected batch")
	}
	q.committed = true
	return nil
}

func (q *replayQueue) Pending() (int64, int64) {
	if q.committed {
		return 0, 0
	}
	return int64(len(q.batch.Records)), q.batch.Bytes
}

// TestRecorderWritesNoncompliantTopicToQueue 验证弱 Topic 不会收到新的直写请求。
func TestRecorderWritesNoncompliantTopicToQueue(t *testing.T) {
	publisher := &stubPublisher{}
	queue := &stubQueue{}
	topic := NewTopicContract(ReliabilityProduction)
	topic.Update(TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := NewRecorder(publisher, topic, queue, discardLogger())

	if err := recorder.Write(t.Context(), []*alsv1.RequestRecord{{Id: "record-1"}}); err != nil {
		t.Fatalf("Recorder.Write(noncompliant topic) error = %v, want nil", err)
	}
	if publisher.calls != 0 {
		t.Errorf("Publisher.Publish() calls = %d, want 0", publisher.calls)
	}
	if queue.written != 1 {
		t.Errorf("RecordQueue.Write() records = %d, want 1", queue.written)
	}
}

// TestRecorderQueuesWholePartiallyPublishedBatch 验证部分确认后仍把整批写入 WAL。
func TestRecorderQueuesWholePartiallyPublishedBatch(t *testing.T) {
	publisher := &stubPublisher{
		result: PublishResult{
			Confirmed: 1,
			Failed:    1,
			Class:     PublishUncertain,
			Err:       errors.New("publish result uncertain"),
		},
	}
	queue := &stubQueue{}
	topic := NewTopicContract(ReliabilityDevelopment)
	topic.Update(TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := NewRecorder(publisher, topic, queue, discardLogger())
	records := []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}}

	if err := recorder.Write(t.Context(), records); err != nil {
		t.Fatalf("Recorder.Write(partial result) error = %v, want nil", err)
	}
	if publisher.calls != 1 {
		t.Errorf("Publisher.Publish() calls = %d, want 1", publisher.calls)
	}
	if queue.written != len(records) {
		t.Errorf("RecordQueue.Write() records = %d, want %d", queue.written, len(records))
	}
}

// TestRecorderRejectsWhenKafkaAndQueueFail 验证主链路与兜底链路同时失败时返回完整错误并计数。
func TestRecorderRejectsWhenKafkaAndQueueFail(t *testing.T) {
	kafkaErr := errors.New("kafka unavailable")
	queueErr := errors.New("disk unavailable")
	publisher := &stubPublisher{result: PublishResult{
		Failed: 1,
		Class:  PublishTemporary,
		Err:    kafkaErr,
	}}
	queue := &stubQueue{writeErr: queueErr}
	topic := NewTopicContract(ReliabilityDevelopment)
	topic.Update(TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := NewRecorder(publisher, topic, queue, discardLogger())

	err := recorder.Write(t.Context(), []*alsv1.RequestRecord{{Id: "record-1"}})
	if !errors.Is(err, kafkaErr) || !errors.Is(err, queueErr) {
		t.Fatalf("Recorder.Write() error = %v, want Kafka and disk queue errors", err)
	}
	if got := recorder.Counters(); got.Accepted != 0 || got.Rejected != 1 {
		t.Errorf("Recorder.Counters() = %+v, want accepted 0 and rejected 1", got)
	}
	if got := recorder.Status(); got.KafkaWritable || got.QueueWritable || !got.Spooling {
		t.Errorf("Recorder.Status() = %+v, want unavailable Kafka and queue with spooling enabled", got)
	}
}

// TestRecorderReplaysThenResumesKafka 验证积压批次确认后才恢复 Kafka 直写。
func TestRecorderReplaysThenResumesKafka(t *testing.T) {
	records := []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}}
	publisher := &stubPublisher{result: PublishResult{Confirmed: len(records)}}
	queue := &replayQueue{batch: QueuedBatch{
		Records:      records,
		LastSequence: 2,
		Bytes:        20,
	}}
	topic := NewTopicContract(ReliabilityDevelopment)
	topic.Update(TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := NewRecorder(publisher, topic, queue, discardLogger())

	replayed, err := recorder.ReplayBatch(t.Context(), len(records))
	if err != nil || !replayed {
		t.Fatalf("Recorder.ReplayBatch() = (%t, %v), want (true, nil)", replayed, err)
	}
	if !queue.committed {
		t.Fatal("RecordQueue.Commit() was not called after Kafka confirmation")
	}

	replayed, err = recorder.ReplayBatch(t.Context(), len(records))
	if err != nil || replayed {
		t.Fatalf("Recorder.ReplayBatch(empty queue) = (%t, %v), want (false, nil)", replayed, err)
	}
	if got := recorder.Status(); got.Spooling || !got.KafkaWritable || !got.QueueWritable {
		t.Errorf("Recorder.Status() = %+v, want Kafka publishing restored", got)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
