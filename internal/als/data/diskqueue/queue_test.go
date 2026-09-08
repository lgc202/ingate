package diskqueue

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tidwall/wal"
	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/als/biz"
	"github.com/lgc202/ingate/internal/als/conf"
)

const testSegmentBytes = 1 << 20

const lockProbePathEnv = "INGATE_ALS_TEST_LOCK_PATH"

// TestQueuePersistsUncommittedRecords 验证读取不删除记录，只有确认后才推进持久化队首。
func TestQueuePersistsUncommittedRecords(t *testing.T) {
	path := t.TempDir()
	records := []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}, {Id: "record-3"}}
	wantBytes := encodedSize(records)
	queue, closeQueue := openQueue(t, path, testSegmentBytes*2)

	if err := queue.Write(t.Context(), records[:2]); err != nil {
		t.Fatalf("Queue.Write(first batch) error = %v, want nil", err)
	}
	if err := queue.Write(t.Context(), records[2:]); err != nil {
		t.Fatalf("Queue.Write(second batch) error = %v, want nil", err)
	}
	if gotRecords, gotBytes := queue.Pending(); gotRecords != 3 || gotBytes != wantBytes {
		t.Fatalf("Queue.Pending() = (%d, %d), want (3, %d)", gotRecords, gotBytes, wantBytes)
	}
	if last, err := queue.log.LastIndex(); err != nil || last != 2 {
		t.Fatalf("wal.Log.LastIndex() = (%d, %v), want (2, nil)", last, err)
	}

	batch, err := queue.Read(t.Context(), 1)
	if err != nil {
		t.Fatalf("Queue.Read() error = %v, want nil", err)
	}
	assertRecordIDs(t, batch.Records, "record-1", "record-2")

	again, err := queue.Read(t.Context(), 1)
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

// TestQueueRejectsEmptyBatch 验证空批次不会占用 WAL 序号或被视为可靠接收。
func TestQueueRejectsEmptyBatch(t *testing.T) {
	queue, _ := openQueue(t, t.TempDir(), testSegmentBytes*2)

	if err := queue.Write(t.Context(), nil); err == nil {
		t.Fatal("Queue.Write(empty batch) error = nil, want non-nil")
	}
	if records, bytes := queue.Pending(); records != 0 || bytes != 0 {
		t.Errorf("Queue.Pending() after empty batch = (%d, %d), want (0, 0)", records, bytes)
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

// TestQueueRejectsLegacyRecords 验证旧的逐记录格式不会被当作版本化 QueueEntry 读取。
func TestQueueRejectsLegacyRecords(t *testing.T) {
	path := t.TempDir()
	log, err := wal.Open(path, &wal.Options{
		SegmentSize: testSegmentBytes,
		LogFormat:   wal.Binary,
		AllowEmpty:  true,
	})
	if err != nil {
		t.Fatalf("wal.Open() error = %v, want nil", err)
	}
	value, err := proto.Marshal(&alsv1.RequestRecord{Id: "legacy-record"})
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v, want nil", err)
	}
	if err := log.Write(1, value); err != nil {
		t.Fatalf("wal.Log.Write() error = %v, want nil", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("wal.Log.Close() error = %v, want nil", err)
	}

	queue, err := NewQueue(queueConfig(path, testSegmentBytes*2))
	if err == nil {
		_ = queue.Close()
		t.Fatal("NewQueue(legacy record) error = nil, want non-nil")
	}
}

// TestQueueOwnsDirectoryExclusively 验证同一 WAL 目录不能被两个 Queue 同时打开。
func TestQueueOwnsDirectoryExclusively(t *testing.T) {
	path := t.TempDir()
	_, closeFirst := openQueue(t, path, testSegmentBytes*2)

	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestQueueDirectoryLockProbe$")
	command.Env = append(os.Environ(), lockProbePathEnv+"="+path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("directory lock subprocess error = %v, want nil; output:\n%s", err, output)
	}

	closeFirst()
	second, err := NewQueue(queueConfig(path, testSegmentBytes*2))
	if err != nil {
		t.Fatalf("NewQueue(released directory) error = %v, want nil", err)
	}
	if err := second.Close(); err != nil {
		t.Errorf("Queue.Close() error = %v, want nil", err)
	}
}

// TestQueueDirectoryLockProbe 在子进程中验证目录锁无法被重复获取。
func TestQueueDirectoryLockProbe(t *testing.T) {
	path := os.Getenv(lockProbePathEnv)
	if path == "" {
		t.Skip("lock probe runs only as a subprocess")
	}

	queue, err := NewQueue(queueConfig(path, testSegmentBytes*2))
	if err == nil {
		_ = queue.Close()
		t.Fatal("NewQueue(directory owned by another process) error = nil, want non-nil")
	}
	if !errors.Is(err, unix.EWOULDBLOCK) {
		t.Fatalf("NewQueue(directory owned by another process) error = %v, want %v", err, unix.EWOULDBLOCK)
	}
}

// TestQueueRestrictsStoragePermissions 验证已有目录和 WAL 文件会收紧到仅进程用户可访问。
func TestQueueRestrictsStoragePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("os.Mkdir() error = %v, want nil", err)
	}
	legacyFile := filepath.Join(path, "legacy.tmp")
	if err := os.WriteFile(legacyFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v, want nil", err)
	}

	queue, _ := openQueue(t, path, testSegmentBytes*2)
	if err := queue.Write(t.Context(), []*alsv1.RequestRecord{{Id: "record-1"}}); err != nil {
		t.Fatalf("Queue.Write() error = %v, want nil", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(queue directory) error = %v, want nil", err)
	}
	if got := info.Mode().Perm(); got != directoryMode {
		t.Errorf("queue directory permissions = %o, want %o", got, directoryMode)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v, want nil", err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Errorf("DirEntry.Info(%q) error = %v, want nil", entry.Name(), err)
			continue
		}
		if info.Mode().IsRegular() && info.Mode().Perm() != fileMode {
			t.Errorf("queue file %q permissions = %o, want %o", entry.Name(), info.Mode().Perm(), fileMode)
		}
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
