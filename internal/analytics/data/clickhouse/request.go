package clickhouse

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/durationpb"

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
func (s *Store) SaveRequestBatch(ctx context.Context, facts []request.Fact) error {
	if len(facts) == 0 {
		return nil
	}
	writeCtx, cancel := context.WithTimeout(ctx, s.writeTimeout)
	defer cancel()
	if err := s.saveModelCalls(writeCtx, facts); err != nil {
		return err
	}
	return s.saveRequestRecords(writeCtx, facts)
}

func (s *Store) saveRequestRecords(ctx context.Context, facts []request.Fact) (err error) {
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

	for _, fact := range facts {
		record := fact.Record
		if err := batch.Append(
			record.GetId(),
			record.GetRequestId(),
			record.GetStartedAt().AsTime(),
			durationNanoseconds(record.GetDuration()),
			record.GetClientIp(),
			record.GetMethod(),
			record.GetHost(),
			record.GetPath(),
			uint16(record.GetStatusCode()),
			uint8(fact.StatusClass),
			record.GetRequestBytes(),
			record.GetResponseBytes(),
			record.GetGatewayId(),
			record.GetRouteId(),
			record.GetUpstreamId(),
			record.GetCallerId(),
			record.GetAccessKeyId(),
			record.GetEnvoyNodeId(),
			record.GetProtocol(),
			record.GetResponseCodeDetails(),
			uint16(record.GetUpstreamAttempts()),
			record.GetUpstreamAddress(),
			durationNanoseconds(record.GetTimeToFirstByte()),
		); err != nil {
			return fmt.Errorf("append request record %q: %w", record.GetId(), err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send request record batch: %w", err)
	}
	return nil
}

func durationNanoseconds(duration *durationpb.Duration) *uint64 {
	if duration == nil {
		return nil
	}
	nanoseconds := uint64(duration.AsDuration())
	return &nanoseconds
}
