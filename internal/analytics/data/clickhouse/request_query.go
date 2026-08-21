package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
)

// requestSummaryColumns 只读取列表展示所需列，避免翻页时扫描完整详情
const requestSummaryColumns = `
    id,
    started_at,
    duration_ns,
    method,
    host,
    path,
    status_code,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id`

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
		"SELECT %s FROM %s FINAL WHERE started_at >= fromUnixTimestamp64Nano(?) AND started_at < fromUnixTimestamp64Nano(?)",
		requestSummaryColumns,
		s.requestTable,
	)
	args := []any{options.Filter.StartTime.UnixNano(), options.Filter.EndTime.UnixNano()}
	args = appendRequestFilters(&statement, args, options)
	statement.WriteString(" ORDER BY started_at DESC, id DESC LIMIT ?")
	args = append(args, options.PageSize+1)

	rows, err := s.connection.Query(queryCtx, statement.String(), args...)
	if err != nil {
		return request.Page{}, fmt.Errorf("query request records: %w", err)
	}
	records, err := readRequestSummaries(rows, options.PageSize+1)
	if err != nil {
		return request.Page{}, err
	}

	page.Records = records
	if len(records) > options.PageSize {
		last := records[options.PageSize-1]
		page.Records = records[:options.PageSize]
		page.NextCursor = &request.Cursor{StartedAt: last.StartedAt, ID: last.ID}
	}
	modelCalls, err := s.listModelCalls(queryCtx, page.Records)
	if err != nil {
		return request.Page{}, err
	}
	for i := range page.Records {
		page.Records[i].ModelCall = modelCalls[page.Records[i].ID]
	}
	return page, nil
}

// GetRequest 使用完整排序键读取单条请求记录
//
// started_at 既限定保留分区，也与 id 组成查询条件，避免仅按哈希 ID 扫描全部明细
func (s *Store) GetRequest(
	ctx context.Context,
	id string,
	startedAt time.Time,
) (record *request.Record, err error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()
	queryCtx = clickhousego.Context(queryCtx, clickhousego.WithSettings(clickhousego.Settings{
		"do_not_merge_across_partitions_select_final": 1,
	}))

	record, err = s.queryRequestRecord(queryCtx, id, startedAt)
	if err != nil {
		return nil, err
	}
	modelCall, err := s.getModelCall(queryCtx, id, startedAt)
	if err != nil {
		return nil, err
	}
	record.ModelCall = modelCall
	return record, nil
}

func (s *Store) queryRequestRecord(
	ctx context.Context,
	id string,
	startedAt time.Time,
) (record *request.Record, err error) {
	statement := fmt.Sprintf(
		"SELECT %s FROM %s FINAL WHERE started_at = @started_at AND id = @id LIMIT 1",
		requestRecordColumns,
		s.requestTable,
	)
	rows, err := s.connection.Query(
		ctx,
		statement,
		clickhousego.DateNamed("started_at", startedAt, clickhousego.NanoSeconds),
		clickhousego.Named("id", id),
	)
	if err != nil {
		return nil, fmt.Errorf("query request record %q: %w", id, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close request record rows: %w", closeErr))
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("read request record %q: %w", id, err)
		}
		return nil, request.ErrNotFound
	}
	return scanRequestRecord(rows)
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
	if filter.CallerID != "" {
		statement.WriteString(" AND caller_id = ?")
		args = append(args, filter.CallerID)
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
	if filter.StatusClass == request.StatusClassNoResponse {
		statement.WriteString(" AND status_code = 0")
	} else if filter.StatusClass != request.StatusClassUnknown {
		statement.WriteString(" AND status_class = ?")
		args = append(args, uint8(filter.StatusClass))
	}
	if filter.StatusCode != nil {
		statement.WriteString(" AND status_code = ?")
		args = append(args, *filter.StatusCode)
	}
	if options.Cursor != nil {
		statement.WriteString(" AND (started_at < fromUnixTimestamp64Nano(?) OR (started_at = fromUnixTimestamp64Nano(?) AND id < ?))")
		cursorNanoseconds := options.Cursor.StartedAt.UnixNano()
		args = append(args, cursorNanoseconds, cursorNanoseconds, options.Cursor.ID)
	}
	return args
}

// scanRequestSummary 把 ClickHouse 的紧凑数值类型还原为列表查询摘要
func scanRequestSummary(rows driver.Rows) (request.Summary, error) {
	var (
		summary    request.Summary
		durationNS *uint64
		statusCode uint16
	)
	if err := rows.Scan(
		&summary.ID,
		&summary.StartedAt,
		&durationNS,
		&summary.Method,
		&summary.Host,
		&summary.Path,
		&statusCode,
		&summary.GatewayID,
		&summary.RouteID,
		&summary.UpstreamID,
		&summary.CallerID,
		&summary.AccessKeyID,
	); err != nil {
		return request.Summary{}, fmt.Errorf("scan request summary: %w", err)
	}
	summary.Duration = durationValue(durationNS)
	summary.StatusCode = statusCode
	return summary, nil
}

func readRequestSummaries(rows driver.Rows, capacity int) (records []request.Summary, err error) {
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close request record rows: %w", closeErr))
		}
	}()
	records = make([]request.Summary, 0, capacity)
	for rows.Next() {
		record, scanErr := scanRequestSummary(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read request record rows: %w", err)
	}
	return records, nil
}

// scanRequestRecord 把 ClickHouse 的紧凑数值类型还原为请求领域记录
func scanRequestRecord(rows driver.Rows) (*request.Record, error) {
	var (
		record            request.Record
		durationNS        *uint64
		statusClass       uint8
		timeToFirstByteNS *uint64
	)
	if err := rows.Scan(
		&record.ID,
		&record.RequestID,
		&record.StartedAt,
		&durationNS,
		&record.ClientIP,
		&record.Method,
		&record.Host,
		&record.Path,
		&record.StatusCode,
		&statusClass,
		&record.RequestBytes,
		&record.ResponseBytes,
		&record.GatewayID,
		&record.RouteID,
		&record.UpstreamID,
		&record.CallerID,
		&record.AccessKeyID,
		&record.EnvoyNodeID,
		&record.Protocol,
		&record.ResponseCodeDetails,
		&record.UpstreamAttempts,
		&record.UpstreamAddress,
		&timeToFirstByteNS,
	); err != nil {
		return nil, fmt.Errorf("scan request record: %w", err)
	}
	record.Duration = durationValue(durationNS)
	record.StatusClass = request.StatusClass(statusClass)
	record.TimeToFirstByte = durationValue(timeToFirstByteNS)
	return &record, nil
}

func durationValue(nanoseconds *uint64) *time.Duration {
	if nanoseconds == nil {
		return nil
	}
	duration := time.Duration(*nanoseconds)
	return &duration
}
