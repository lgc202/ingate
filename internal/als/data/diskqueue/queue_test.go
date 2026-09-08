package diskqueue

import (
	"errors"
	"testing"

	"github.com/tidwall/wal"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

const testSegmentBytes = 1 << 20

// TestQueuePersistsUncommittedRecords 验证读取不删除记录，只有确认后才推进持久化队首。
func TestQueuePersistsUncommittedRecords(t *testing.T) {
	path := t.TempDir()
	records := []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}, {Id: "record-3"}}
	wantBytes := encodedSize(records)
	queue, closeQueue := openQueue(t, path, testSegmentBytes*2)

	if err := queue.Write(t.Context(), records); err != nil {
		t.Fatalf("Queue.Write() error = %v, want nil", err)
	}
	if gotRecords, gotBytes := queue.Pending(); gotRecords != 3 || gotBytes != wantBytes {
		t.Fatalf("Queue.Pending() = (%d, %d), want (3, %d)", gotRecords, gotBytes, wantBytes)
	}

	batch, err := queue.Read(t.Context(), 2)
	if err != nil {
		t.Fatalf("Queue.Read() error = %v, want nil", err)
	}
	assertRecordIDs(t, batch.Records, "record-1", "record-2")

	again, err := queue.Read(t.Context(), 2)
	if err != nil {
		t.Fatalf("Queue.Read() before commit error = %v, want nil", err)
	}
	assertRecordIDs(t, again.Records, "record-1", "record-2")

	if err := queue.Commit(t.Context(), batch); err != nil {
		t.Fatalf("Queue.Commit() error = %v, want nil", err)
	}
	closeQueue()

	queue, _ = openQueue(t, path, testSegmentBytes*2)
	if gotRecords, gotBytes := queue.Pending(); gotRecords != 1 || gotBytes != int64(proto.Size(records[2])) {
		t.Fatalf("Queue.Pending() after reopen = (%d, %d), want (1, %d)", gotRecords, gotBytes, proto.Size(records[2]))
	}
	batch, err = queue.Read(t.Context(), 10)
	if err != nil {
		t.Fatalf("Queue.Read() after reopen error = %v, want nil", err)
	}
	assertRecordIDs(t, batch.Records, "record-3")
	if err := queue.Commit(t.Context(), batch); err != nil {
		t.Fatalf("Queue.Commit() final batch error = %v, want nil", err)
	}
	if _, err := queue.Read(t.Context(), 1); !errors.Is(err, biz.ErrQueueEmpty) {
		t.Fatalf("Queue.Read() after final commit error = %v, want %v", err, biz.ErrQueueEmpty)
	}
}

// TestQueueEnforcesCapacityAtomically 验证超出容量的批次不会留下部分记录。
func TestQueueEnforcesCapacityAtomically(t *testing.T) {
	record := &alsv1.RequestRecord{Id: "record-1"}
	queue, _ := openQueue(t, t.TempDir(), int64(proto.Size(record)))

	if err := queue.Write(t.Context(), []*alsv1.RequestRecord{record, record}); !errors.Is(err, errFull) {
		t.Fatalf("Queue.Write(oversized batch) error = %v, want %v", err, errFull)
	}
	if records, bytes := queue.Pending(); records != 0 || bytes != 0 {
		t.Fatalf("Queue.Pending() after rejected batch = (%d, %d), want (0, 0)", records, bytes)
	}

	if err := queue.Write(t.Context(), []*alsv1.RequestRecord{record}); err != nil {
		t.Fatalf("Queue.Write(capacity boundary) error = %v, want nil", err)
	}
	if err := queue.Write(t.Context(), []*alsv1.RequestRecord{record}); !errors.Is(err, errFull) {
		t.Fatalf("Queue.Write(full queue) error = %v, want %v", err, errFull)
	}
}

// TestQueueRejectsInvalidCommit 验证错误的确认元数据不会推进队首或破坏待处理计数。
func TestQueueRejectsInvalidCommit(t *testing.T) {
	record := &alsv1.RequestRecord{Id: "record-1"}
	queue, _ := openQueue(t, t.TempDir(), testSegmentBytes*2)
	if err := queue.Write(t.Context(), []*alsv1.RequestRecord{record}); err != nil {
		t.Fatalf("Queue.Write() error = %v, want nil", err)
	}

	batch, err := queue.Read(t.Context(), 1)
	if err != nil {
		t.Fatalf("Queue.Read() error = %v, want nil", err)
	}
	batch.Bytes++
	if err := queue.Commit(t.Context(), batch); err == nil {
		t.Fatal("Queue.Commit(invalid metadata) error = nil, want non-nil")
	}
	if records, bytes := queue.Pending(); records != 1 || bytes != int64(proto.Size(record)) {
		t.Errorf("Queue.Pending() after rejected commit = (%d, %d), want (1, %d)", records, bytes, proto.Size(record))
	}
}

// TestQueueRejectsCorruptRecordsOnOpen 验证无法解析的持久化记录会阻止队列启动。
func TestQueueRejectsCorruptRecordsOnOpen(t *testing.T) {
	path := t.TempDir()
	log, err := wal.Open(path, &wal.Options{
		SegmentSize: testSegmentBytes,
		LogFormat:   wal.Binary,
		AllowEmpty:  true,
	})
	if err != nil {
		t.Fatalf("wal.Open() error = %v, want nil", err)
	}
	if err := log.Write(1, []byte{0xff}); err != nil {
		t.Fatalf("wal.Log.Write() error = %v, want nil", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("wal.Log.Close() error = %v, want nil", err)
	}

	queue, err := NewQueue(queueConfig(path, testSegmentBytes*2))
	if err == nil {
		_ = queue.Close()
		t.Fatal("NewQueue(corrupt record) error = nil, want non-nil")
	}
}

func openQueue(t *testing.T, path string, maxBytes int64) (*Queue, func()) {
	t.Helper()

	queue, err := NewQueue(queueConfig(path, maxBytes))
	if err != nil {
		t.Fatalf("NewQueue() error = %v, want nil", err)
	}
	closed := false
	closeQueue := func() {
		if closed {
			return
		}
		closed = true
		if err := queue.Close(); err != nil {
			t.Errorf("Queue.Close() error = %v, want nil", err)
		}
	}
	t.Cleanup(closeQueue)
	return queue, closeQueue
}

func queueConfig(path string, maxBytes int64) *conf.Data_DiskQueue {
	return &conf.Data_DiskQueue{
		Path:         path,
		SegmentBytes: testSegmentBytes,
		Sync:         true,
		MaxBytes:     maxBytes,
	}
}

func encodedSize(records []*alsv1.RequestRecord) int64 {
	var size int64
	for _, record := range records {
		size += int64(proto.Size(record))
	}
	return size
}

func assertRecordIDs(t *testing.T, records []*alsv1.RequestRecord, want ...string) {
	t.Helper()

	if len(records) != len(want) {
		t.Fatalf("record count = %d, want %d", len(records), len(want))
	}
	for i, record := range records {
		if record.GetId() != want[i] {
			t.Errorf("record[%d].id = %q, want %q", i, record.GetId(), want[i])
		}
	}
}
