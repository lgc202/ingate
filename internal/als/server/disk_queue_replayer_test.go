package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

type replayPublisher struct {
	publish func(context.Context, []*alsv1.RequestRecord) biz.PublishResult
}

type replayerQueue struct {
	mu        sync.Mutex
	records   []*alsv1.RequestRecord
	committed bool
}

func (p replayPublisher) Publish(ctx context.Context, records []*alsv1.RequestRecord) biz.PublishResult {
	return p.publish(ctx, records)
}

func (q *replayerQueue) Write(_ context.Context, records []*alsv1.RequestRecord) error {
	q.mu.Lock()
	q.records = append(q.records, records...)
	q.mu.Unlock()
	return nil
}

func (q *replayerQueue) Read(context.Context, int) (biz.QueuedBatch, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.committed || len(q.records) == 0 {
		return biz.QueuedBatch{}, biz.ErrQueueEmpty
	}
	return biz.QueuedBatch{
		Records:      append([]*alsv1.RequestRecord(nil), q.records...),
		LastSequence: uint64(len(q.records)),
	}, nil
}

func (q *replayerQueue) Commit(_ context.Context, batch biz.QueuedBatch) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if batch.LastSequence != uint64(len(q.records)) {
		return errors.New("unexpected replay batch")
	}
	q.committed = true
	return nil
}

func (q *replayerQueue) Pending() (int64, int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.committed {
		return 0, 0
	}
	return int64(len(q.records)), 0
}

// TestDiskQueueReplayerRetriesThenDrains 验证短暂失败退避后会连续排空队列并恢复直写。
func TestDiskQueueReplayerRetriesThenDrains(t *testing.T) {
	queue := &replayerQueue{records: []*alsv1.RequestRecord{{Id: "record-1"}}}
	results := []biz.PublishResult{
		{Failed: 1, Class: biz.PublishTemporary, Err: errors.New("Kafka unavailable")},
		{Confirmed: 1},
	}
	publisher := replayPublisher{publish: func(context.Context, []*alsv1.RequestRecord) biz.PublishResult {
		result := results[0]
		results = results[1:]
		return result
	}}
	recorder := newReplayerRecorder(publisher, queue)
	replayer := NewDiskQueueReplayer(replayerConfig(), recorder, discardServerLogger())

	delay, paused := replayer.replay(t.Context())
	if paused || delay < 10*time.Millisecond || delay > 15*time.Millisecond {
		t.Fatalf("DiskQueueReplayer.replay() = (%s, %t), want initial retry range", delay, paused)
	}

	delay, paused = replayer.replay(t.Context())
	if paused || delay != 10*time.Millisecond {
		t.Fatalf("DiskQueueReplayer.replay() after recovery = (%s, %t), want (10ms, false)", delay, paused)
	}
	if pending, _ := queue.Pending(); pending != 0 {
		t.Errorf("RecordQueue.Pending() = %d, want 0", pending)
	}
	if status := recorder.Status(); status.Spooling || !status.KafkaWritable {
		t.Errorf("Recorder.Status() = %+v, want Kafka direct publishing restored", status)
	}
}

// TestDiskQueueReplayerReportsPermanentFailureAfterRetry 验证临时失败日志不会遮蔽后续的永久失败。
func TestDiskQueueReplayerReportsPermanentFailureAfterRetry(t *testing.T) {
	queue := &replayerQueue{records: []*alsv1.RequestRecord{{Id: "record-1"}}}
	results := []biz.PublishResult{
		{Failed: 1, Class: biz.PublishTemporary, Err: errors.New("Kafka unavailable")},
		{Failed: 1, Class: biz.PublishPermanent, Err: errors.New("record is invalid")},
	}
	publisher := replayPublisher{publish: func(context.Context, []*alsv1.RequestRecord) biz.PublishResult {
		result := results[0]
		results = results[1:]
		return result
	}}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	replayer := NewDiskQueueReplayer(
		replayerConfig(),
		newReplayerRecorder(publisher, queue),
		logger,
	)

	if _, paused := replayer.replay(t.Context()); paused {
		t.Fatal("DiskQueueReplayer.replay() paused after a temporary failure = true, want false")
	}
	if _, paused := replayer.replay(t.Context()); !paused {
		t.Fatal("DiskQueueReplayer.replay() paused after a permanent failure = false, want true")
	}
	if got := logs.String(); !strings.Contains(got, "disk queue replay paused") {
		t.Errorf("DiskQueueReplayer logs = %q, want permanent failure message", got)
	}
}

// TestDiskQueueReplayerStopsAfterPermanentFailure 验证永久错误暂停队首后只等待进程停止。
func TestDiskQueueReplayerStopsAfterPermanentFailure(t *testing.T) {
	queue := &replayerQueue{records: []*alsv1.RequestRecord{{Id: "record-1"}}}
	called := make(chan struct{}, 1)
	var calls atomic.Int32
	publisher := replayPublisher{publish: func(context.Context, []*alsv1.RequestRecord) biz.PublishResult {
		calls.Add(1)
		select {
		case called <- struct{}{}:
		default:
		}
		return biz.PublishResult{
			Failed: 1,
			Class:  biz.PublishPermanent,
			Err:    errors.New("record is invalid"),
		}
	}}
	replayer := NewDiskQueueReplayer(
		replayerConfig(),
		newReplayerRecorder(publisher, queue),
		discardServerLogger(),
	)
	done := startReplayer(t, replayer)
	awaitReplayCall(t, called)
	stopReplayer(t, replayer, done)

	if got := calls.Load(); got != 1 {
		t.Errorf("Publisher.Publish() calls = %d, want 1", got)
	}
	if pending, _ := queue.Pending(); pending != 1 {
		t.Errorf("RecordQueue.Pending() = %d, want 1", pending)
	}
}

// TestDiskQueueReplayerCancelsInFlightPublish 验证停止时取消在途 Kafka 写入且不确认 WAL。
func TestDiskQueueReplayerCancelsInFlightPublish(t *testing.T) {
	queue := &replayerQueue{records: []*alsv1.RequestRecord{{Id: "record-1"}}}
	called := make(chan struct{}, 1)
	publisher := replayPublisher{publish: func(ctx context.Context, _ []*alsv1.RequestRecord) biz.PublishResult {
		select {
		case called <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return biz.PublishResult{Failed: 1, Class: biz.PublishTemporary, Err: ctx.Err()}
	}}
	replayer := NewDiskQueueReplayer(
		replayerConfig(),
		newReplayerRecorder(publisher, queue),
		discardServerLogger(),
	)
	done := startReplayer(t, replayer)
	awaitReplayCall(t, called)
	stopReplayer(t, replayer, done)

	if pending, _ := queue.Pending(); pending != 1 {
		t.Errorf("RecordQueue.Pending() = %d, want 1", pending)
	}
}

// TestReplayBackoffBounds 验证指数退避和抖动始终位于配置边界内。
func TestReplayBackoffBounds(t *testing.T) {
	const (
		minBackoff = 10 * time.Millisecond
		maxBackoff = 80 * time.Millisecond
	)

	backoff := replayBackoff{
		min:  minBackoff,
		max:  maxBackoff,
		next: minBackoff,
	}
	for attempt, base := range []time.Duration{
		minBackoff,
		2 * minBackoff,
		4 * minBackoff,
		maxBackoff,
		maxBackoff,
	} {
		delay := backoff.nextDelay()
		upper := min(base+base/2, maxBackoff)
		if delay < base || delay > upper {
			t.Errorf("replayBackoff.nextDelay() attempt %d = %s, want [%s, %s]", attempt+1, delay, base, upper)
		}
	}

	backoff.reset()
	if delay := backoff.nextDelay(); delay < minBackoff || delay > minBackoff+minBackoff/2 {
		t.Errorf("replayBackoff.nextDelay() after reset = %s, want reset range", delay)
	}
}

func newReplayerRecorder(publisher biz.RecordPublisher, queue biz.RecordQueue) *biz.Recorder {
	topic := biz.NewTopicContract(biz.ReliabilityDevelopment)
	topic.Update(biz.TopicTopology{Exists: true, ReplicationFactor: 1, MinInSyncReplicas: 1})
	return biz.NewRecorder(publisher, topic, queue, discardServerLogger())
}

func replayerConfig() *conf.Data_DiskQueue {
	return &conf.Data_DiskQueue{
		ReplayBatchSize:  10,
		ReplayMinBackoff: durationpb.New(10 * time.Millisecond),
		ReplayMaxBackoff: durationpb.New(80 * time.Millisecond),
	}
}

func startReplayer(t *testing.T, replayer *DiskQueueReplayer) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- replayer.Start(t.Context())
	}()
	return done
}

func awaitReplayCall(t *testing.T, called <-chan struct{}) {
	t.Helper()
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("Publisher.Publish() was not called")
	}
}

func stopReplayer(t *testing.T, replayer *DiskQueueReplayer, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := replayer.Stop(ctx); err != nil {
		t.Fatalf("DiskQueueReplayer.Stop() error = %v, want nil", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("DiskQueueReplayer.Start() error = %v, want nil", err)
		}
	case <-ctx.Done():
		t.Fatal("DiskQueueReplayer.Start() did not return after Stop")
	}
}
