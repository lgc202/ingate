package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
)

// requestRecordColumns 是请求事实表的完整列契约
//
// 写入和详情查询共用这个顺序，新增列时必须同时调整 Append 与 Scan
const requestRecordColumns = `
    id,
    request_id,
    started_at,
    duration_ns,
    client_ip,
    method,
    host,
    path,
    status_code,
    status_class,
    request_bytes,
    response_bytes,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id,
    envoy_node_id,
    protocol,
    response_code_details,
    upstream_attempts,
    upstream_address,
    time_to_first_byte_ns`

// SaveRequestBatch 批量保存请求事实
//
// 模型调用先写入独立事实表，请求记录成功后才会从查询入口暴露这些数据
// 任一步失败都由 Kafka 重投，两个 ReplacingMergeTree 使用稳定事件 ID 收敛重复明细
func (s *Store) SaveRequestBatch(ctx context.Context, records []request.Record) error {
	if len(records) == 0 {
		return nil
	}
	writeCtx, cancel := context.WithTimeout(ctx, s.writeTimeout)
	defer cancel()
	if err := s.saveModelCalls(writeCtx, records); err != nil {
		return err
	}
	return s.saveRequestRecords(writeCtx, records)
}

func (s *Store) saveRequestRecords(ctx context.Context, records []request.Record) (err error) {
	statement := fmt.Sprintf("INSERT INTO %s (%s)", s.requestTable, requestRecordColumns)
	batch, err := s.connection.PrepareBatch(ctx, statement)
	if err != nil {
		return fmt.Errorf("prepare request record batch: %w", err)
	}
	defer func() {
		if batch.IsSent() {
			return
		}
		if abortErr := batch.Abort(); abortErr != nil {
			err = errors.Join(err, fmt.Errorf("abort request record batch: %w", abortErr))
		}
	}()

	for i := range records {
		record := &records[i]
		if err := batch.Append(
			record.ID,
			record.RequestID,
			record.StartedAt,
			durationNanoseconds(record.Duration),
			record.ClientIP,
			record.Method,
			record.Host,
			record.Path,
			record.StatusCode,
			uint8(record.StatusClass),
			record.RequestBytes,
			record.ResponseBytes,
			record.GatewayID,
			record.RouteID,
			record.UpstreamID,
			record.CallerID,
			record.AccessKeyID,
			record.EnvoyNodeID,
			record.Protocol,
			record.ResponseCodeDetails,
			record.UpstreamAttempts,
			record.UpstreamAddress,
			durationNanoseconds(record.TimeToFirstByte),
		); err != nil {
			return fmt.Errorf("append request record %q: %w", record.ID, err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send request record batch: %w", err)
	}
	return nil
}

func durationNanoseconds(duration *time.Duration) *uint64 {
	if duration == nil {
		return nil
	}
	nanoseconds := uint64(*duration)
	return &nanoseconds
}
