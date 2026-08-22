package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/internal/analytics/biz/aiusage"
)

// aiUsageAggregates 合并 SummingMergeTree 中跨分钟或跨数据 Part 的加法指标
//
// 查询必须继续使用 sum，不能依赖后台 Part 合并已经完成
const aiUsageAggregates = `
    sum(call_count) AS call_count,
    sum(normal_response_count) AS normal_response_count,
    sum(token_reported_call_count) AS token_reported_call_count,
    sum(input_token_count) AS input_token_count,
    sum(output_token_count) AS output_token_count,
    sum(total_token_count) AS total_token_count`

// QueryAIUsageSummary 查询整个时间范围的模型调用与 Token 汇总
func (s *Store) QueryAIUsageSummary(ctx context.Context, filter aiusage.Filter) (aiusage.Metrics, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var statement strings.Builder
	fmt.Fprintf(
		&statement,
		"SELECT %s FROM %s WHERE started_at >= ? AND started_at < ?",
		aiUsageAggregates,
		s.modelUsageTable,
	)
	args := []any{filter.StartTime, filter.EndTime}
	args = appendAIUsageFilters(&statement, args, filter)

	var metrics aiusage.Metrics
	if err := s.connection.QueryRow(queryCtx, statement.String(), args...).Scan(
		&metrics.CallCount,
		&metrics.NormalResponseCount,
		&metrics.TokenReportedCallCount,
		&metrics.InputTokens,
		&metrics.OutputTokens,
		&metrics.TotalTokens,
	); err != nil {
		return aiusage.Metrics{}, fmt.Errorf("query AI usage summary: %w", err)
	}
	return metrics, nil
}

// QueryAIUsageTrend 按时间粒度查询模型调用与 Token 趋势
func (s *Store) QueryAIUsageTrend(
	ctx context.Context,
	query aiusage.TrendQuery,
) (points []aiusage.TrendPoint, err error) {
	bucket, err := aiUsageTimeBucketExpression(query.Bucket)
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
		aiUsageAggregates,
		s.modelUsageTable,
	)
	args := []any{query.Filter.StartTime, query.Filter.EndTime}
	args = appendAIUsageFilters(&statement, args, query.Filter)
	statement.WriteString(" GROUP BY bucket ORDER BY bucket")

	rows, err := s.connection.Query(queryCtx, statement.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query AI usage trend: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close AI usage trend rows: %w", closeErr))
		}
	}()

	for rows.Next() {
		var point aiusage.TrendPoint
		if err := rows.Scan(
			&point.StartedAt,
			&point.Metrics.CallCount,
			&point.Metrics.NormalResponseCount,
			&point.Metrics.TokenReportedCallCount,
			&point.Metrics.InputTokens,
			&point.Metrics.OutputTokens,
			&point.Metrics.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("scan AI usage trend: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read AI usage trend rows: %w", err)
	}
	return points, nil
}

// QueryAIUsageBreakdown 按受控业务维度查询模型调用与 Token 分布
func (s *Store) QueryAIUsageBreakdown(
	ctx context.Context,
	query aiusage.BreakdownQuery,
) (items []aiusage.BreakdownItem, err error) {
	dimension, err := aiUsageDimensionColumn(query.Dimension)
	if err != nil {
		return nil, err
	}
	order, err := aiUsageBreakdownOrder(query.Order)
	if err != nil {
		return nil, err
	}
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	var statement strings.Builder
	fmt.Fprintf(
		&statement,
		"SELECT %s AS dimension_value, %s FROM %s WHERE started_at >= ? AND started_at < ?",
		dimension,
		aiUsageAggregates,
		s.modelUsageTable,
	)
	args := []any{query.Filter.StartTime, query.Filter.EndTime}
	args = appendAIUsageFilters(&statement, args, query.Filter)
	fmt.Fprintf(&statement, " GROUP BY dimension_value ORDER BY %s LIMIT ?", order)
	args = append(args, query.Limit)

	rows, err := s.connection.Query(queryCtx, statement.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("query AI usage breakdown: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close AI usage breakdown rows: %w", closeErr))
		}
	}()

	items = make([]aiusage.BreakdownItem, 0, query.Limit)
	for rows.Next() {
		var item aiusage.BreakdownItem
		if err := rows.Scan(
			&item.DimensionValue,
			&item.Metrics.CallCount,
			&item.Metrics.NormalResponseCount,
			&item.Metrics.TokenReportedCallCount,
			&item.Metrics.InputTokens,
			&item.Metrics.OutputTokens,
			&item.Metrics.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("scan AI usage breakdown: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read AI usage breakdown rows: %w", err)
	}
	return items, nil
}

func appendAIUsageFilters(statement *strings.Builder, args []any, filter aiusage.Filter) []any {
	if filter.GatewayID != "" {
		statement.WriteString(" AND gateway_id = ?")
		args = append(args, filter.GatewayID)
	}
	if filter.CallerID != "" {
		statement.WriteString(" AND caller_id = ?")
		args = append(args, filter.CallerID)
	}
	if filter.RouteID != "" {
		statement.WriteString(" AND route_id = ?")
		args = append(args, filter.RouteID)
	}
	if filter.ClientModel != "" {
		statement.WriteString(" AND client_model = ?")
		args = append(args, filter.ClientModel)
	}
	if filter.UpstreamID != "" {
		statement.WriteString(" AND upstream_id = ?")
		args = append(args, filter.UpstreamID)
	}
	if filter.UpstreamModel != "" {
		statement.WriteString(" AND upstream_model = ?")
		args = append(args, filter.UpstreamModel)
	}
	return args
}

func aiUsageTimeBucketExpression(bucket aiusage.TimeBucket) (string, error) {
	switch bucket {
	case aiusage.TimeBucketMinute:
		return "toStartOfMinute(started_at)", nil
	case aiusage.TimeBucketFiveMinutes:
		return "toStartOfInterval(started_at, INTERVAL 5 MINUTE)", nil
	case aiusage.TimeBucketHour:
		return "toStartOfHour(started_at)", nil
	case aiusage.TimeBucketDay:
		return "toStartOfDay(started_at)", nil
	default:
		return "", fmt.Errorf("unsupported AI usage time bucket %d", bucket)
	}
}

func aiUsageDimensionColumn(dimension aiusage.Dimension) (string, error) {
	switch dimension {
	case aiusage.DimensionCaller:
		return "caller_id", nil
	case aiusage.DimensionRoute:
		return "route_id", nil
	case aiusage.DimensionClientModel:
		return "client_model", nil
	case aiusage.DimensionUpstream:
		return "upstream_id", nil
	case aiusage.DimensionUpstreamModel:
		return "upstream_model", nil
	default:
		return "", fmt.Errorf("unsupported AI usage dimension %d", dimension)
	}
}

func aiUsageBreakdownOrder(order aiusage.BreakdownOrder) (string, error) {
	switch order {
	case aiusage.BreakdownOrderCallCount:
		return "call_count DESC, dimension_value", nil
	case aiusage.BreakdownOrderTotalTokens:
		return "total_token_count DESC, call_count DESC, dimension_value", nil
	default:
		return "", fmt.Errorf("unsupported AI usage breakdown order %d", order)
	}
}
