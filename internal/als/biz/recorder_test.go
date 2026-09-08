package biz

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

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
	commitErr error
	committed bool
	blocked   bool
}

type publishCall struct {
	records []*alsv1.RequestRecord
	result  chan PublishResult
}

type controlledPublisher struct {
	calls chan publishCall
}

type concurrentQueue struct {
	mu      sync.Mutex
	records []*alsv1.RequestRecord
	writes  chan string
}

type replayCallResult struct {
	result ReplayResult
	err    error
}

func (p *stubPublisher) Publish(context.Context, []*alsv1.RequestRecord) PublishResult {
	p.calls++
	return p.result
}

func (p *controlledPublisher) Publish(_ context.Context, records []*alsv1.RequestRecord) PublishResult {
	result := make(chan PublishResult, 1)
	p.calls <- publishCall{records: records, result: result}
	return <-result
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

func (q *stubQueue) Status() QueueStatus {
	records, bytes := q.Pending()
	if errors.Is(q.writeErr, ErrQueueFull) {
		return QueueStatus{
			State:          QueueBlocked,
			PendingRecords: records,
			PendingBytes:   bytes,
		}
	}
	return writableQueueStatus(records, bytes)
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
	if q.commitErr != nil {
		return q.commitErr
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

func (q *replayQueue) Status() QueueStatus {
	records, bytes := q.Pending()
	if q.blocked {
		return QueueStatus{
			State:          QueueBlocked,
			PendingRecords: records,
			PendingBytes:   bytes,
		}
	}
	return writableQueueStatus(records, bytes)
}

func (q *concurrentQueue) Write(_ context.Context, records []*alsv1.RequestRecord) error {
	q.mu.Lock()
	q.records = append(q.records, records...)
	q.mu.Unlock()

	for _, record := range records {
		q.writes <- record.GetId()
	}
	return nil
}

func (q *concurrentQueue) Read(context.Context, int) (QueuedBatch, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.records) == 0 {
		return QueuedBatch{}, ErrQueueEmpty
	}
	return QueuedBatch{
		Records:      append([]*alsv1.RequestRecord(nil), q.records...),
		LastSequence: uint64(len(q.records)),
		Bytes:        int64(len(q.records) * 10),
	}, nil
}

func (q *concurrentQueue) Commit(_ context.Context, batch QueuedBatch) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if batch.LastSequence != uint64(len(q.records)) {
		return errors.New("unexpected batch")
	}
	q.records = nil
	return nil
}

func (q *concurrentQueue) Pending() (int64, int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(len(q.records)), int64(len(q.records) * 10)
}

func (q *concurrentQueue) Status() QueueStatus {
	records, bytes := q.Pending()
	return writableQueueStatus(records, bytes)
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
	if got := recorder.Counters(); got != (RecorderCounters{
		Valid:         2,
		KafkaAccepted: 1,
		Spooled:       2,
	}) {
		t.Errorf("Recorder.Counters() = %+v, want partial Kafka acceptance and a fully spooled batch", got)
	}
}

// TestRecorderEstablishesFailureBarrier 验证故障后的新批次不再访问 Kafka。
func TestRecorderEstablishesFailureBarrier(t *testing.T) {
	publisher := &controlledPublisher{calls: make(chan publishCall)}
	queue := &concurrentQueue{writes: make(chan string, 3)}
	topic := NewTopicContract(ReliabilityDevelopment)
	topic.Update(TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	recorder := NewRecorder(publisher, topic, queue, discardLogger())

	firstDone := writeAsync(recorder, "record-1")
	first := awaitPublish(t, publisher.calls)
	secondDone := writeAsync(recorder, "record-2")
	second := awaitPublish(t, publisher.calls)

	first.result <- PublishResult{
		Failed: 1,
		Class:  PublishTemporary,
		Err:    errors.New("Kafka unavailable"),
	}
	if got := awaitWrite(t, queue.writes); got != "record-1" {
		t.Fatalf("RecordQueue.Write() record = %q, want record-1", got)
	}

	thirdDone := writeAsync(recorder, "record-3")
	select {
	case call := <-publisher.calls:
		call.result <- PublishResult{Confirmed: 1}
		t.Error("Publisher.Publish() received a post-barrier batch")
	case got := <-queue.writes:
		if got != "record-3" {
			t.Errorf("RecordQueue.Write() record = %q, want record-3", got)
		}
	case <-time.After(time.Second):
		t.Fatal("Recorder.Write() did not choose a target after the failure barrier")
	}

	second.result <- PublishResult{Confirmed: 1}
	for i, done := range []<-chan error{firstDone, secondDone, thirdDone} {
		if err := awaitWriteResult(t, done); err != nil {
			t.Errorf("Recorder.Write() call %d error = %v, want nil", i+1, err)
		}
	}

	status := recorder.Status()
	if !status.Spooling || status.Queue.PendingRecords != 2 {
		t.Errorf("Recorder.Status() = %+v, want spooling with two queued records", status)
	}

	replayDone := replayAsync(recorder, 3)
	replay := awaitPublish(t, publisher.calls)
	if got, want := recordIDs(replay.records), []string{"record-1", "record-3"}; !slices.Equal(got, want) {
		t.Errorf("replayed records = %q, want %q", got, want)
	}
	replay.result <- PublishResult{Confirmed: 2}
	if got := awaitReplayResult(t, replayDone); got.err != nil || got.result != ReplayCommitted {
		t.Fatalf("Recorder.ReplayBatch() = (%v, %v), want (%v, nil)", got.result, got.err, ReplayCommitted)
	}
	if result, err := recorder.ReplayBatch(t.Context(), 3); err != nil || result != ReplayIdle {
		t.Fatalf("Recorder.ReplayBatch(empty queue) = (%v, %v), want (%v, nil)", result, err, ReplayIdle)
	}

	fourthDone := writeAsync(recorder, "record-4")
	fourth := awaitPublish(t, publisher.calls)
	fourth.result <- PublishResult{Confirmed: 1}
	if err := awaitWriteResult(t, fourthDone); err != nil {
		t.Fatalf("Recorder.Write() after recovery error = %v, want nil", err)
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
	if got := recorder.Counters(); got.Valid != 1 || got.Rejected != 1 {
		t.Errorf("Recorder.Counters() = %+v, want one valid and rejected record", got)
	}
	if got := recorder.Status(); got.KafkaWritable || got.Queue.Writable || !got.Spooling {
		t.Errorf("Recorder.Status() = %+v, want unavailable Kafka and queue with spooling enabled", got)
	}
}

// TestRecorderCapacityStatusRecovers 验证容量拒绝不会锁存为 I/O 故障，释放空间后状态可自行恢复。
func TestRecorderCapacityStatusRecovers(t *testing.T) {
	queue := &stubQueue{writeErr: ErrQueueFull}
	topic := NewTopicContract(ReliabilityDevelopment)
	recorder := NewRecorder(&stubPublisher{}, topic, queue, discardLogger())

	if err := recorder.Write(t.Context(), []*alsv1.RequestRecord{{Id: "record-1"}}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Recorder.Write(full queue) error = %v, want %v", err, ErrQueueFull)
	}
	if status := recorder.Status(); status.Queue.Writable {
		t.Errorf("Recorder.Status() after capacity rejection = %+v, want queue unavailable", status)
	}

	queue.writeErr = nil
	if status := recorder.Status(); !status.Queue.Writable {
		t.Errorf("Recorder.Status() after capacity recovery = %+v, want queue available", status)
	}
}

// TestRecorderInvalidBatchPreservesQueueHealth 验证单批约束错误不会被记成 WAL I/O 故障。
func TestRecorderInvalidBatchPreservesQueueHealth(t *testing.T) {
	queue := &stubQueue{writeErr: ErrQueueInvalidBatch}
	recorder := NewRecorder(
		&stubPublisher{},
		NewTopicContract(ReliabilityDevelopment),
		queue,
		discardLogger(),
	)

	if err := recorder.Write(t.Context(), []*alsv1.RequestRecord{{Id: "record-1"}}); !errors.Is(err, ErrQueueInvalidBatch) {
		t.Fatalf("Recorder.Write(invalid batch) error = %v, want %v", err, ErrQueueInvalidBatch)
	}
	if status := recorder.Status(); !status.Queue.Writable {
		t.Errorf("Recorder.Status() after invalid batch = %+v, want queue available", status)
	}
}

// TestRecorderCountsDiscardedRecords 验证协议边界丢弃记录的累计计数。
func TestRecorderCountsDiscardedRecords(t *testing.T) {
	recorder := newTestRecorder(&stubPublisher{}, &stubQueue{})

	recorder.Discard(2)
	recorder.Discard(0)

	if got := recorder.Counters(); got.Discarded != 2 || got.Valid != 0 {
		t.Errorf("Recorder.Counters() = %+v, want two discarded records", got)
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

	result, err := recorder.ReplayBatch(t.Context(), len(records))
	if err != nil || result != ReplayCommitted {
		t.Fatalf("Recorder.ReplayBatch() = (%v, %v), want (%v, nil)", result, err, ReplayCommitted)
	}
	if !queue.committed {
		t.Fatal("RecordQueue.Commit() was not called after Kafka confirmation")
	}
	if got := recorder.Counters(); got.KafkaAccepted != 2 || got.Replayed != 2 || got.Committed != 2 {
		t.Errorf("Recorder.Counters() = %+v, want two accepted, replayed, and committed records", got)
	}

	result, err = recorder.ReplayBatch(t.Context(), len(records))
	if err != nil || result != ReplayIdle {
		t.Fatalf("Recorder.ReplayBatch(empty queue) = (%v, %v), want (%v, nil)", result, err, ReplayIdle)
	}
	if got := recorder.Status(); got.Spooling || !got.KafkaWritable || !got.Queue.Writable {
		t.Errorf("Recorder.Status() = %+v, want Kafka publishing restored", got)
	}
}

// TestRecorderKeepsPartiallyPublishedReplay 验证 Kafka 部分确认时不提交整个 WAL 条目。
func TestRecorderKeepsPartiallyPublishedReplay(t *testing.T) {
	publishErr := errors.New("publish result uncertain")
	publisher := &stubPublisher{result: PublishResult{
		Confirmed: 1,
		Failed:    1,
		Class:     PublishUncertain,
		Err:       publishErr,
	}}
	queue := replayQueueWithRecords("record-1", "record-2")
	recorder := newTestRecorder(publisher, queue)

	result, err := recorder.ReplayBatch(t.Context(), 2)
	if result != ReplayRetry || !errors.Is(err, publishErr) {
		t.Fatalf("Recorder.ReplayBatch() = (%v, %v), want (%v, publish error)", result, err, ReplayRetry)
	}
	if queue.committed {
		t.Fatal("RecordQueue.Commit() was called after a partial Kafka result")
	}
	if got := recorder.Counters(); got.KafkaAccepted != 1 || got.Replayed != 1 || got.Committed != 0 {
		t.Errorf("Recorder.Counters() = %+v, want one accepted replay without a commit", got)
	}
}

// TestRecorderPausesPermanentReplayFailure 验证永久错误保留队首并禁止自动恢复直写。
func TestRecorderPausesPermanentReplayFailure(t *testing.T) {
	publishErr := errors.New("record is invalid")
	publisher := &stubPublisher{result: PublishResult{
		Failed: 1,
		Class:  PublishPermanent,
		Err:    publishErr,
	}}
	queue := replayQueueWithRecords("record-1")
	recorder := newTestRecorder(publisher, queue)

	result, err := recorder.ReplayBatch(t.Context(), 1)
	if result != ReplayPaused || !errors.Is(err, publishErr) {
		t.Fatalf("Recorder.ReplayBatch() = (%v, %v), want (%v, publish error)", result, err, ReplayPaused)
	}
	if queue.committed {
		t.Fatal("RecordQueue.Commit() was called after a permanent Kafka failure")
	}
	if got := recorder.Status(); !got.Spooling {
		t.Errorf("Recorder.Status() = %+v, want spooling after a permanent failure", got)
	}
}

// TestRecorderRetriesAfterQueueCommitFailure 验证 Kafka 已确认但 WAL 确认失败时保留条目。
func TestRecorderRetriesAfterQueueCommitFailure(t *testing.T) {
	commitErr := errors.New("commit unavailable")
	publisher := &stubPublisher{result: PublishResult{Confirmed: 1}}
	queue := replayQueueWithRecords("record-1")
	queue.commitErr = commitErr
	recorder := newTestRecorder(publisher, queue)

	for attempt := 1; attempt <= 2; attempt++ {
		result, err := recorder.ReplayBatch(t.Context(), 1)
		if result != ReplayRetry || !errors.Is(err, commitErr) {
			t.Fatalf("Recorder.ReplayBatch() attempt %d = (%v, %v), want (%v, commit error)",
				attempt, result, err, ReplayRetry)
		}
	}
	if publisher.calls != 2 {
		t.Errorf("Publisher.Publish() calls = %d, want 2", publisher.calls)
	}
	if queue.committed {
		t.Fatal("queue entry was removed after commit failures")
	}
	if got := recorder.Counters(); got.KafkaAccepted != 2 || got.Replayed != 2 || got.Committed != 0 {
		t.Errorf("Recorder.Counters() = %+v, want duplicate replay attempts without a commit", got)
	}
}

// TestRecorderReplaysBlockedQueue 验证容量阻塞只拒绝新追加，不妨碍回放释放旧积压。
func TestRecorderReplaysBlockedQueue(t *testing.T) {
	queue := replayQueueWithRecords("record-1")
	queue.blocked = true
	recorder := newTestRecorder(&stubPublisher{result: PublishResult{Confirmed: 1}}, queue)

	result, err := recorder.ReplayBatch(t.Context(), 10)
	if err != nil || result != ReplayCommitted {
		t.Fatalf("Recorder.ReplayBatch(blocked queue) = (%v, %v), want (%v, nil)", result, err, ReplayCommitted)
	}
	if !queue.committed {
		t.Fatal("RecordQueue.Commit() was not called while queue capacity was blocked")
	}
}

func writeAsync(recorder *Recorder, id string) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- recorder.Write(context.Background(), []*alsv1.RequestRecord{{Id: id}})
	}()
	return done
}

func replayAsync(recorder *Recorder, limit int) <-chan replayCallResult {
	done := make(chan replayCallResult, 1)
	go func() {
		result, err := recorder.ReplayBatch(context.Background(), limit)
		done <- replayCallResult{result: result, err: err}
	}()
	return done
}

func awaitPublish(t *testing.T, calls <-chan publishCall) publishCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(time.Second):
		t.Fatal("Publisher.Publish() was not called")
		return publishCall{}
	}
}

func awaitWrite(t *testing.T, writes <-chan string) string {
	t.Helper()
	select {
	case id := <-writes:
		return id
	case <-time.After(time.Second):
		t.Fatal("RecordQueue.Write() was not called")
		return ""
	}
}

func awaitWriteResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		t.Fatal("Recorder.Write() did not return")
		return nil
	}
}

func awaitReplayResult(t *testing.T, done <-chan replayCallResult) replayCallResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(time.Second):
		t.Fatal("Recorder.ReplayBatch() did not return")
		return replayCallResult{}
	}
}

func recordIDs(records []*alsv1.RequestRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.GetId())
	}
	return ids
}

func replayQueueWithRecords(ids ...string) *replayQueue {
	records := make([]*alsv1.RequestRecord, 0, len(ids))
	for _, id := range ids {
		records = append(records, &alsv1.RequestRecord{Id: id})
	}
	return &replayQueue{batch: QueuedBatch{
		Records:      records,
		LastSequence: uint64(len(records)),
		Bytes:        int64(len(records) * 10),
	}}
}

func newTestRecorder(publisher RecordPublisher, queue RecordQueue) *Recorder {
	topic := NewTopicContract(ReliabilityDevelopment)
	topic.Update(TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	return NewRecorder(publisher, topic, queue, discardLogger())
}

func writableQueueStatus(records, bytes int64) QueueStatus {
	return QueueStatus{
		State:          QueueHealthy,
		Writable:       true,
		PendingRecords: records,
		PendingBytes:   bytes,
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
