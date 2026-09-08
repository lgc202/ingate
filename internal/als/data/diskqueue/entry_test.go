package diskqueue

import (
	"bytes"
	"hash/crc32"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

// TestQueueEntryRoundTrip 验证持久格式保留批次、稳定记录 ID 和 W3C Trace Context。
func TestQueueEntryRoundTrip(t *testing.T) {
	traceState, err := trace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatalf("trace.ParseTraceState() error = %v, want nil", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{17, 18, 19, 20, 21, 22, 23, 24},
		TraceFlags: trace.FlagsSampled,
		TraceState: traceState,
	})
	ctx := trace.ContextWithSpanContext(t.Context(), spanContext)
	enqueuedAt := time.Unix(1_725_689_600, 123)
	records := []*alsv1.RequestRecord{{Id: "record-1"}, {Id: "record-2"}}

	value, payloadBytes, err := encodeEntry(ctx, records, enqueuedAt)
	if err != nil {
		t.Fatalf("encodeEntry() error = %v, want nil", err)
	}
	again, _, err := encodeEntry(ctx, records, enqueuedAt)
	if err != nil {
		t.Fatalf("encodeEntry() second call error = %v, want nil", err)
	}
	if !bytes.Equal(value, again) {
		t.Error("encodeEntry() returned different deterministic encodings")
	}

	entry := new(QueueEntry)
	if err := proto.Unmarshal(value, entry); err != nil {
		t.Fatalf("proto.Unmarshal(QueueEntry) error = %v, want nil", err)
	}
	if entry.GetFormatVersion() != currentFormatVersion {
		t.Errorf("QueueEntry.format_version = %d, want %d", entry.GetFormatVersion(), currentFormatVersion)
	}
	if entry.GetEnqueuedAtUnixNano() != enqueuedAt.UnixNano() {
		t.Errorf("QueueEntry.enqueued_at_unix_nano = %d, want %d", entry.GetEnqueuedAtUnixNano(), enqueuedAt.UnixNano())
	}
	if entry.GetTraceparent() != "00-0102030405060708090a0b0c0d0e0f10-1112131415161718-01" {
		t.Errorf("QueueEntry.traceparent = %q, want W3C span context", entry.GetTraceparent())
	}
	if entry.GetTracestate() != traceState.String() {
		t.Errorf("QueueEntry.tracestate = %q, want %q", entry.GetTracestate(), traceState.String())
	}

	decoded, gotBytes, err := decodeEntry(value)
	if err != nil {
		t.Fatalf("decodeEntry() error = %v, want nil", err)
	}
	assertRecordIDs(t, decoded, "record-1", "record-2")
	if gotBytes != payloadBytes || gotBytes != encodedSize(records) {
		t.Errorf("decodeEntry() payload bytes = %d, want %d", gotBytes, payloadBytes)
	}
}

// TestQueueEntryDetectsRecordPayloadBitChanges 验证任一记录 payload 位翻转都会被 CRC32C 拒绝。
func TestQueueEntryDetectsRecordPayloadBitChanges(t *testing.T) {
	record := &alsv1.RequestRecord{Id: "stable-record-id", Path: "/v1/chat/completions"}
	value, _, err := encodeEntry(t.Context(), []*alsv1.RequestRecord{record}, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("encodeEntry() error = %v, want nil", err)
	}
	recordValue, err := proto.Marshal(record)
	if err != nil {
		t.Fatalf("proto.Marshal(RequestRecord) error = %v, want nil", err)
	}
	offset := bytes.Index(value, recordValue)
	if offset < 0 {
		t.Fatal("encoded QueueEntry does not contain the request record payload")
	}

	for i := range recordValue {
		for bit := range 8 {
			corrupt := bytes.Clone(value)
			corrupt[offset+i] ^= byte(1 << bit)
			if _, _, err := decodeEntry(corrupt); err == nil {
				t.Fatalf("decodeEntry(payload byte %d bit %d changed) error = nil, want non-nil", i, bit)
			}
		}
	}
}

// TestQueueEntryRejectsUnknownVersion 验证有效校验和也不能绕过持久格式版本边界。
func TestQueueEntryRejectsUnknownVersion(t *testing.T) {
	entry := &QueueEntry{
		FormatVersion:      currentFormatVersion + 1,
		EnqueuedAtUnixNano: time.Now().UnixNano(),
		Records:            [][]byte{{1}},
	}
	value := marshalTestEntry(t, entry)

	if _, _, err := decodeEntry(value); err == nil {
		t.Fatal("decodeEntry(unknown version) error = nil, want non-nil")
	}
}

// TestQueueEntryRejectsInvalidRecords 验证空批次、空记录、nil 和超限记录不会进入 WAL。
func TestQueueEntryRejectsInvalidRecords(t *testing.T) {
	tests := []struct {
		name    string
		records []*alsv1.RequestRecord
	}{
		{name: "empty_batch"},
		{name: "nil_record", records: []*alsv1.RequestRecord{nil}},
		{name: "empty_record", records: []*alsv1.RequestRecord{{}}},
		{
			name: "oversized_record",
			records: []*alsv1.RequestRecord{{
				Id:   "record-1",
				Path: strings.Repeat("x", requestrecord.MaxEncodedBytes),
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := encodeEntry(t.Context(), test.records, time.Unix(1, 0)); err == nil {
				t.Fatal("encodeEntry(invalid records) error = nil, want non-nil")
			}
		})
	}
}

func marshalTestEntry(t *testing.T, entry *QueueEntry) []byte {
	t.Helper()

	marshal := proto.MarshalOptions{Deterministic: true}
	value, err := marshal.Marshal(entry)
	if err != nil {
		t.Fatalf("proto.MarshalOptions.Marshal(checksum input) error = %v, want nil", err)
	}
	entry.Crc32C = crc32.Checksum(value, castagnoliTable)
	value, err = marshal.Marshal(entry)
	if err != nil {
		t.Fatalf("proto.MarshalOptions.Marshal(QueueEntry) error = %v, want nil", err)
	}
	return value
}
