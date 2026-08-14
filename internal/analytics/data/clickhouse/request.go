package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/analytics/biz/request"
)

// requestColumns 同时用于 INSERT 和 SELECT，顺序必须与 Append 及 Scan 一致
const requestColumns = `
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
    envoy_node_id,
    protocol,
    response_code_details,
    upstream_attempts,
    upstream_address,
    time_to_first_byte_ns`

// SaveRequestBatch 批量保存请求事实
//
// idempotencyKey 在完全相同的 facts 重试时必须保持不变，使常见的
// “ClickHouse 已写入但上游位点未提交”重试不会重复更新请求明细和分钟聚合视图
//
// 该机制是 At Least Once 下的幂等重试，不宣称跨任意批次边界的 Exactly Once
func (s *Store) SaveRequestBatch(ctx context.Context, idempotencyKey string, facts []request.Fact) (err error) {
	if len(facts) == 0 {
		return nil
	}
	writeCtx, cancel := context.WithTimeout(ctx, s.writeTimeout)
	defer cancel()
	// Kafka 只有在入库成功后才提交 offset，两步之间崩溃会重投同一批消息
	// 稳定 Token 让明细表拒绝重复 Block；第二个设置让依赖的分钟物化视图同样去重
	// 它们只对内容和顺序完全相同的重试有效，不等于 Kafka 与 ClickHouse 的分布式事务
	writeCtx = clickhousego.Context(writeCtx, clickhousego.WithSettings(clickhousego.Settings{
		"insert_deduplication_token":                         idempotencyKey,
		"deduplicate_blocks_in_dependent_materialized_views": 1,
	}))

	statement := fmt.Sprintf("INSERT INTO %s (%s)", s.requestTable, requestColumns)
	batch, err := s.connection.PrepareBatch(writeCtx, statement)
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

// ListRequests 按时间和 ID 倒序分页查询短期保留的请求明细
//
// FINAL 保证 ReplacingMergeTree 尚未后台合并时也只返回一个请求事实；时间和 ID
// 游标避免高页码 OFFSET 在 ClickHouse 中重复扫描旧记录
func (s *Store) ListRequests(ctx context.Context, options request.ListOptions) (page request.Page, err error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	// 同一请求的重投记录保持 started_at 不变，必然落在同一月分区；FINAL 可以安全地
	// 分区内并行去重，无需跨分区合并全部结果
	queryCtx = clickhousego.Context(queryCtx, clickhousego.WithSettings(clickhousego.Settings{
		"do_not_merge_across_partitions_select_final": 1,
	}))

	var statement strings.Builder
	fmt.Fprintf(
		&statement,
		"SELECT %s FROM %s FINAL WHERE started_at >= ? AND started_at < ?",
		requestColumns,
		s.requestTable,
	)
	args := []any{options.Filter.StartTime, options.Filter.EndTime}
	args = appendRequestFilters(&statement, args, options)
	statement.WriteString(" ORDER BY started_at DESC, id DESC LIMIT ?")
	args = append(args, options.PageSize+1)

	rows, err := s.connection.Query(queryCtx, statement.String(), args...)
	if err != nil {
		return request.Page{}, fmt.Errorf("query request records: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close request record rows: %w", closeErr))
		}
	}()

	records := make([]*alsv1.RequestRecord, 0, options.PageSize+1)
	for rows.Next() {
		record, scanErr := scanRequestRecord(rows)
		if scanErr != nil {
			return request.Page{}, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return request.Page{}, fmt.Errorf("read request record rows: %w", err)
	}

	page.Records = records
	if len(records) > options.PageSize {
		last := records[options.PageSize-1]
		page.Records = records[:options.PageSize]
		page.NextCursor = &request.Cursor{StartedAt: last.GetStartedAt().AsTime(), ID: last.GetId()}
	}
	return page, nil
}

// appendRequestFilters 只拼接预定义列，所有用户输入继续作为 ClickHouse 参数传递
func appendRequestFilters(statement *strings.Builder, args []any, options request.ListOptions) []any {
	filter := options.Filter
	if filter.GatewayID != "" {
		statement.WriteString(" AND gateway_id = ?")
		args = append(args, filter.GatewayID)
	}
	if filter.RouteID != "" {
		statement.WriteString(" AND route_id = ?")
		args = append(args, filter.RouteID)
	}
	if filter.UpstreamID != "" {
		statement.WriteString(" AND upstream_id = ?")
		args = append(args, filter.UpstreamID)
	}
	if filter.RequestID != "" {
		statement.WriteString(" AND request_id = ?")
		args = append(args, filter.RequestID)
	}
	if filter.Method != "" {
		statement.WriteString(" AND method = ?")
		args = append(args, filter.Method)
	}
	if filter.Host != "" {
		statement.WriteString(" AND host = ?")
		args = append(args, filter.Host)
	}
	if filter.PathPrefix != "" {
		statement.WriteString(" AND startsWith(path, ?)")
		args = append(args, filter.PathPrefix)
	}
	if filter.StatusClass != request.StatusClassUnknown {
		statement.WriteString(" AND status_class = ?")
		args = append(args, uint8(filter.StatusClass))
	}
	if filter.StatusCode != nil {
		statement.WriteString(" AND status_code = ?")
		args = append(args, *filter.StatusCode)
	}
	if options.Cursor != nil {
		statement.WriteString(" AND (started_at < ? OR (started_at = ? AND id < ?))")
		args = append(args, options.Cursor.StartedAt, options.Cursor.StartedAt, options.Cursor.ID)
	}
	return args
}

// scanRequestRecord 把 ClickHouse 的紧凑数值类型还原为公共 ALS Proto 类型
func scanRequestRecord(rows driver.Rows) (*alsv1.RequestRecord, error) {
	var (
		record            alsv1.RequestRecord
		startedAt         time.Time
		durationNS        *uint64
		statusCode        uint16
		statusClass       uint8
		upstreamAttempts  uint16
		timeToFirstByteNS *uint64
	)
	if err := rows.Scan(
		&record.Id,
		&record.RequestId,
		&startedAt,
		&durationNS,
		&record.ClientIp,
		&record.Method,
		&record.Host,
		&record.Path,
		&statusCode,
		&statusClass,
		&record.RequestBytes,
		&record.ResponseBytes,
		&record.GatewayId,
		&record.RouteId,
		&record.UpstreamId,
		&record.EnvoyNodeId,
		&record.Protocol,
		&record.ResponseCodeDetails,
		&upstreamAttempts,
		&record.UpstreamAddress,
		&timeToFirstByteNS,
	); err != nil {
		return nil, fmt.Errorf("scan request record: %w", err)
	}
	record.StartedAt = timestamppb.New(startedAt)
	record.Duration = protobufDuration(durationNS)
	record.StatusCode = uint32(statusCode)
	record.UpstreamAttempts = uint32(upstreamAttempts)
	record.TimeToFirstByte = protobufDuration(timeToFirstByteNS)
	return &record, nil
}

func durationNanoseconds(duration *durationpb.Duration) *uint64 {
	if duration == nil {
		return nil
	}
	nanoseconds := uint64(duration.AsDuration())
	return &nanoseconds
}

func protobufDuration(nanoseconds *uint64) *durationpb.Duration {
	if nanoseconds == nil {
		return nil
	}
	return durationpb.New(time.Duration(*nanoseconds))
}
