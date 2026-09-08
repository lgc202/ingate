package diskqueue

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"time"

	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

const currentFormatVersion uint32 = 1

var castagnoliTable = crc32.MakeTable(crc32.Castagnoli)

type decodedEntry struct {
	records     []*alsv1.RequestRecord
	bytes       int64
	enqueuedAt  time.Time
	spanContext oteltrace.SpanContext
}

// encodeEntry 将一个已校验的请求批次封装为带版本和 CRC32C 的持久条目。
func encodeEntry(
	ctx context.Context,
	records []*alsv1.RequestRecord,
	enqueuedAt time.Time,
) ([]byte, int64, error) {
	if len(records) == 0 {
		return nil, 0, errors.New("queue entry must contain at least one request record")
	}
	if enqueuedAt.UnixNano() <= 0 {
		return nil, 0, errors.New("queue entry enqueue time must be after the Unix epoch")
	}

	values, payloadBytes, err := encodeRecords(records)
	if err != nil {
		return nil, 0, err
	}

	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	entry := &QueueEntry{
		FormatVersion:      currentFormatVersion,
		EnqueuedAtUnixNano: enqueuedAt.UnixNano(),
		Records:            values,
		Traceparent:        carrier.Get("traceparent"),
		Tracestate:         carrier.Get("tracestate"),
	}

	checksum, err := entryChecksum(entry)
	if err != nil {
		return nil, 0, err
	}
	entry.Crc32C = checksum

	value, err := proto.MarshalOptions{Deterministic: true}.Marshal(entry)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal queue entry: %w", err)
	}
	return value, payloadBytes, nil
}

// decodeEntry 在解析 RequestRecord 前验证持久格式版本、结构和 CRC32C。
func decodeEntry(value []byte) (decodedEntry, error) {
	entry := new(QueueEntry)
	if err := proto.Unmarshal(value, entry); err != nil {
		return decodedEntry{}, fmt.Errorf("unmarshal queue entry: %w", err)
	}
	if err := validateEntry(entry); err != nil {
		return decodedEntry{}, err
	}

	checksum, err := entryChecksum(entry)
	if err != nil {
		return decodedEntry{}, err
	}
	if checksum != entry.GetCrc32C() {
		return decodedEntry{}, fmt.Errorf("queue entry checksum = %08x, want %08x", checksum, entry.GetCrc32C())
	}

	records, bytes, err := decodeRecords(entry.GetRecords())
	if err != nil {
		return decodedEntry{}, err
	}
	return decodedEntry{
		records:     records,
		bytes:       bytes,
		enqueuedAt:  time.Unix(0, entry.GetEnqueuedAtUnixNano()).UTC(),
		spanContext: extractSpanContext(entry),
	}, nil
}

func extractSpanContext(entry *QueueEntry) oteltrace.SpanContext {
	carrier := propagation.MapCarrier{
		"traceparent": entry.GetTraceparent(),
		"tracestate":  entry.GetTracestate(),
	}
	ctx := propagation.TraceContext{}.Extract(context.Background(), carrier)
	return oteltrace.SpanContextFromContext(ctx)
}

func validateEntry(entry *QueueEntry) error {
	if entry.GetFormatVersion() != currentFormatVersion {
		return fmt.Errorf("unsupported queue entry format version %d", entry.GetFormatVersion())
	}
	if entry.GetEnqueuedAtUnixNano() <= 0 {
		return errors.New("queue entry has an invalid enqueue time")
	}
	if len(entry.GetRecords()) == 0 {
		return errors.New("queue entry contains no request records")
	}
	return nil
}

func encodeRecords(records []*alsv1.RequestRecord) ([][]byte, int64, error) {
	values := make([][]byte, 0, len(records))
	var payloadBytes int64
	for _, record := range records {
		if record == nil {
			return nil, 0, errors.New("queue entry cannot contain a nil request record")
		}

		value, err := proto.Marshal(record)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request record: %w", err)
		}
		if len(value) == 0 {
			return nil, 0, errors.New("queue entry cannot contain an empty request record")
		}
		if len(value) > requestrecord.MaxEncodedBytes {
			return nil, 0, errors.New("queue entry request record exceeds the size limit")
		}

		values = append(values, value)
		payloadBytes += int64(len(value))
	}
	return values, payloadBytes, nil
}

func decodeRecords(values [][]byte) ([]*alsv1.RequestRecord, int64, error) {
	records := make([]*alsv1.RequestRecord, 0, len(values))
	var payloadBytes int64
	for _, value := range values {
		if len(value) == 0 {
			return nil, 0, errors.New("queue entry contains an empty request record")
		}
		if len(value) > requestrecord.MaxEncodedBytes {
			return nil, 0, errors.New("queue entry request record exceeds the size limit")
		}

		record := new(alsv1.RequestRecord)
		if err := proto.Unmarshal(value, record); err != nil {
			return nil, 0, fmt.Errorf("unmarshal request record: %w", err)
		}
		records = append(records, record)
		payloadBytes += int64(len(value))
	}

	return records, payloadBytes, nil
}

func entryChecksum(entry *QueueEntry) (uint32, error) {
	checksumEntry := proto.Clone(entry).(*QueueEntry)
	checksumEntry.Crc32C = 0
	value, err := proto.MarshalOptions{Deterministic: true}.Marshal(checksumEntry)
	if err != nil {
		return 0, fmt.Errorf("marshal queue entry checksum input: %w", err)
	}
	return crc32.Checksum(value, castagnoliTable), nil
}
