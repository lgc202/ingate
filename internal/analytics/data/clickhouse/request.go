package clickhouse

import (
	"context"
	"fmt"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"golang.org/x/sync/errgroup"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
)

// requestRecordColumns 是请求事实表的完整列契约
//
// 写入参数和详情查询共用这个顺序，新增列时必须同时调整写入与 Scan
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

// SaveRequestBatch 保存 Kafka 本轮交付的请求事实。
//
// Kafka Poll 的批次边界在进程重启后可能变化，因此不能拿整批数据生成幂等标识
// 每条请求分别以稳定事件 ID 写入，ClickHouse 再通过 async insert 在服务端合批
// 所有写入收到持久化确认后调用方才会提交 Kafka offset。
func (s *Store) SaveRequestBatch(ctx context.Context, records []request.Record) error {
	if len(records) == 0 {
		return nil
	}
	writeCtx, cancel := context.WithTimeout(ctx, s.writeTimeout)
	defer cancel()

	group, groupCtx := errgroup.WithContext(writeCtx)
	group.SetLimit(s.writeConcurrency)
	var scheduleErr error
	for i := range records {
		if err := groupCtx.Err(); err != nil {
			scheduleErr = err
			break
		}
		record := records[i]
		group.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}
			return s.saveRequest(groupCtx, record)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return scheduleErr
}

func (s *Store) saveRequest(ctx context.Context, record request.Record) error {
	// 模型事实先落库，请求事实最后写入，避免列表短暂暴露缺少模型明细的记录
	if err := s.saveModelCall(ctx, record); err != nil {
		return err
	}
	return s.saveRequestRecord(ctx, record)
}

func (s *Store) saveRequestRecord(ctx context.Context, record request.Record) error {
	statement := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.requestTable,
		requestRecordColumns,
	)
	if err := s.connection.Exec(
		asyncInsertContext(ctx, record.ID),
		statement,
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
		return fmt.Errorf("insert request record %q: %w", record.ID, err)
	}
	return nil
}

// asyncInsertContext 为一次逻辑事件建立 ClickHouse 幂等写入上下文
//
// wait=true 保证服务端完成持久化后才返回。显式事件 ID 让 Kafka 重投仍命中同一个
// 去重标识；dependent materialized view 使用同一标识过滤重投，避免累计指标翻倍
// ClickHouse 不把 token 纳入异步队列键，不同事件仍可在服务端合并为较大的数据块
func asyncInsertContext(ctx context.Context, eventID string) context.Context {
	return clickhousego.Context(
		ctx,
		clickhousego.WithAsync(true),
		clickhousego.WithSettings(clickhousego.Settings{
			"async_insert_deduplicate":                           1,
			"insert_deduplicate":                                 1,
			"insert_deduplication_token":                         eventID,
			"deduplicate_blocks_in_dependent_materialized_views": 1,
		}),
	)
}

func durationNanoseconds(duration *time.Duration) *uint64 {
	if duration == nil {
		return nil
	}
	nanoseconds := uint64(*duration)
	return &nanoseconds
}
