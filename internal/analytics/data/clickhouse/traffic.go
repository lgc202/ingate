package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

// trafficAggregates 合并 AggregatingMergeTree 中跨分钟或跨数据 Part 的聚合状态
const trafficAggregates = `
    sum(request_count) AS request_count,
    sum(client_error_count) AS client_error_count,
    sum(server_error_count) AS server_error_count,
    sum(no_response_count) AS no_response_count,
    toUInt64(round(coalesce(avgMerge(duration_average), 0))) AS average_duration_ns,
    toUInt64(round(coalesce(quantileTDigestMerge(0.5)(duration_p50), 0))) AS p50_duration_ns,
    toUInt64(round(coalesce(quantileTDigestMerge(0.95)(duration_p95), 0))) AS p95_duration_ns,
    toUInt64(round(coalesce(quantileTDigestMerge(0.99)(duration_p99), 0))) AS p99_duration_ns`

// QueryTrafficSummary 查询整个时间范围的流量和延迟汇总
func (s *Store) QueryTrafficSummary(ctx context.Context, filter traffic.Filter) (traffic.Summary, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var statement strings.Builder
	fmt.Fprintf(
		&statement,
		"SELECT %s FROM %s WHERE started_at >= ? AND started_at < ?",
		trafficAggregates,
		s.minuteMetricsTable,
	)
	args := []any{filter.StartTime, filter.EndTime}
	args = appendTrafficFilters(&statement, args, filter)

	var (
		summary           traffic.Summary
		averageDurationNS uint64
		p50DurationNS     uint64
		p95DurationNS     uint64
		p99DurationNS     uint64
	)
	if err := s.connection.QueryRow(queryCtx, statement.String(), args...).Scan(
		&summary.RequestCount,
		&summary.ClientErrors,
		&summary.ServerErrors,
		&summary.NoResponses,
		&averageDurationNS,
		&p50DurationNS,
		&p95DurationNS,
		&p99DurationNS,
	); err != nil {
		return traffic.Summary{}, fmt.Errorf("query traffic summary: %w", err)
	}
	summary.AverageDuration = time.Duration(averageDurationNS)
	summary.P50Duration = time.Duration(p50DurationNS)
	summary.P95Duration = time.Duration(p95DurationNS)
	summary.P99Duration = time.Duration(p99DurationNS)
	return summary, nil
}

// QueryTrafficTrend 从分钟聚合状态查询流量和延迟趋势
//
// 分钟表是唯一预聚合事实，更大的时间粒度在查询时继续合并，避免维护小时表和日表
func (s *Store) QueryTrafficTrend(
	ctx context.Context,
	query traffic.TrendQuery,
) (points []traffic.TrendPoint, err error) {
	bucket, err := timeBucketExpression(query.Bucket)
	if err != nil {
		return nil, err
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var statement strings.Builder
	fmt.Fprintf(
		&statement,
		"SELECT %s AS bucket, %s FROM %s WHERE started_at >= ? AND started_at < ?",
		bucket,
		trafficAggregates,
		s.minuteMetricsTable,
	)
	args := []any{query.Filter.StartTime, query.Filter.EndTime}
	args = appendTrafficFilters(&statement, args, query.Filter)
	statement.WriteString(" GROUP BY bucket ORDER BY bucket")

	rows, err := s.connection.Query(queryCtx, statement.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query traffic trend: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close traffic trend rows: %w", closeErr))
		}
	}()

	for rows.Next() {
		var (
			point             traffic.TrendPoint
			averageDurationNS uint64
			p50DurationNS     uint64
			p95DurationNS     uint64
			p99DurationNS     uint64
		)
		if err := rows.Scan(
			&point.StartedAt,
			&point.RequestCount,
			&point.ClientErrors,
			&point.ServerErrors,
			&point.NoResponses,
			&averageDurationNS,
			&p50DurationNS,
			&p95DurationNS,
			&p99DurationNS,
		); err != nil {
			return nil, fmt.Errorf("scan traffic trend: %w", err)
		}
		point.AverageDuration = time.Duration(averageDurationNS)
		point.P50Duration = time.Duration(p50DurationNS)
		point.P95Duration = time.Duration(p95DurationNS)
		point.P99Duration = time.Duration(p99DurationNS)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read traffic trend rows: %w", err)
	}
	return points, nil
}

// QueryTrafficBreakdown 从分钟聚合状态查询资源维度的流量和延迟分布
//
// Dimension 只会映射到代码内固定列名，资源 ID 和时间范围始终使用查询参数
func (s *Store) QueryTrafficBreakdown(
	ctx context.Context,
	query traffic.BreakdownQuery,
) (items []traffic.BreakdownItem, err error) {
	dimension, err := trafficDimensionColumn(query.Dimension)
	if err != nil {
		return nil, err
	}
	order, err := trafficBreakdownOrder(query.Order)
	if err != nil {
		return nil, err
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var statement strings.Builder
	fmt.Fprintf(
		&statement,
		"SELECT %s AS resource_id, %s FROM %s WHERE started_at >= ? AND started_at < ?",
		dimension,
		trafficAggregates,
		s.minuteMetricsTable,
	)
	args := []any{query.Filter.StartTime, query.Filter.EndTime}
	args = appendTrafficFilters(&statement, args, query.Filter)
	fmt.Fprintf(&statement, " AND %s != ''", dimension)
	fmt.Fprintf(&statement, " GROUP BY resource_id ORDER BY %s LIMIT ?", order)
	args = append(args, query.Limit)

	rows, err := s.connection.Query(queryCtx, statement.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query traffic breakdown: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close traffic breakdown rows: %w", closeErr))
		}
	}()

	items = make([]traffic.BreakdownItem, 0, query.Limit)
	for rows.Next() {
		var (
			item              traffic.BreakdownItem
			averageDurationNS uint64
			p50DurationNS     uint64
			p95DurationNS     uint64
			p99DurationNS     uint64
		)
		if err := rows.Scan(
			&item.ResourceID,
			&item.RequestCount,
			&item.ClientErrors,
			&item.ServerErrors,
			&item.NoResponses,
			&averageDurationNS,
			&p50DurationNS,
			&p95DurationNS,
			&p99DurationNS,
		); err != nil {
			return nil, fmt.Errorf("scan traffic breakdown: %w", err)
		}
		item.AverageDuration = time.Duration(averageDurationNS)
		item.P50Duration = time.Duration(p50DurationNS)
		item.P95Duration = time.Duration(p95DurationNS)
		item.P99Duration = time.Duration(p99DurationNS)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read traffic breakdown rows: %w", err)
	}
	return items, nil
}

// QueryResourceTraffic 只聚合资源列表展示所需的计数，不计算趋势或耗时分位值
func (s *Store) QueryResourceTraffic(
	ctx context.Context,
	query traffic.ResourceTrafficQuery,
) (summaries []traffic.ResourceTrafficSummary, err error) {
	dimension, err := trafficDimensionColumn(query.Dimension)
	if err != nil {
		return nil, err
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	statement := fmt.Sprintf(`
		SELECT %s AS resource_id,
		       sum(request_count) AS request_count,
		       sum(server_error_count) AS server_error_count,
		       sum(no_response_count) AS no_response_count
		FROM %s
		WHERE started_at >= ? AND started_at < ? AND %s IN (?)
		GROUP BY resource_id`, dimension, s.minuteMetricsTable, dimension)
	rows, err := s.connection.Query(
		queryCtx,
		statement,
		query.StartTime,
		query.EndTime,
		query.ResourceIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("query resource traffic: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close resource traffic rows: %w", closeErr))
		}
	}()

	summaries = make([]traffic.ResourceTrafficSummary, 0, len(query.ResourceIDs))
	for rows.Next() {
		var summary traffic.ResourceTrafficSummary
		if err := rows.Scan(
			&summary.ResourceID,
			&summary.RequestCount,
			&summary.ServerErrors,
			&summary.NoResponses,
		); err != nil {
			return nil, fmt.Errorf("scan resource traffic: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read resource traffic rows: %w", err)
	}
	return summaries, nil
}

// appendTrafficFilters 追加 Gateway、Route 和 Upstream 的可选资源过滤条件
func appendTrafficFilters(statement *strings.Builder, args []any, filter traffic.Filter) []any {
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
	return args
}

// timeBucketExpression 把受控枚举映射为 ClickHouse 时间分桶表达式
func timeBucketExpression(bucket traffic.TimeBucket) (string, error) {
	switch bucket {
	case traffic.TimeBucketMinute:
		return "started_at", nil
	case traffic.TimeBucketFiveMinutes:
		return "toStartOfInterval(started_at, INTERVAL 5 MINUTE)", nil
	case traffic.TimeBucketHour:
		return "toStartOfHour(started_at)", nil
	case traffic.TimeBucketDay:
		return "toStartOfDay(started_at)", nil
	default:
		return "", fmt.Errorf("unsupported traffic time bucket %d", bucket)
	}
}

// trafficDimensionColumn 把受控资源维度映射为 ClickHouse 列名
func trafficDimensionColumn(dimension traffic.Dimension) (string, error) {
	switch dimension {
	case traffic.DimensionGateway:
		return "gateway_id", nil
	case traffic.DimensionRoute:
		return "route_id", nil
	case traffic.DimensionUpstream:
		return "upstream_id", nil
	default:
		return "", fmt.Errorf("unsupported traffic dimension %d", dimension)
	}
}

// trafficBreakdownOrder 把受控排序枚举映射为 ClickHouse 表达式
func trafficBreakdownOrder(order traffic.BreakdownOrder) (string, error) {
	switch order {
	case traffic.BreakdownOrderRequestCount:
		return "request_count DESC, resource_id", nil
	case traffic.BreakdownOrderServerErrorRate:
		return "server_error_count / greatest(request_count, 1) DESC, request_count DESC, resource_id", nil
	case traffic.BreakdownOrderP95Duration:
		return "p95_duration_ns DESC, request_count DESC, resource_id", nil
	default:
		return "", fmt.Errorf("unsupported traffic breakdown order %d", order)
	}
}
