package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/analytics/biz/aiusage"
	"github.com/lgc202/ingate/internal/pkg/analyticsconfig"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// aiUsageAggregates 合并 SummingMergeTree 中跨分钟或跨数据 Part 的加法指标。
//
// 查询必须继续使用 sum，不能依赖后台 Part 合并已经完成。
const aiUsageAggregates = `
    sum(call_count) AS call_count,
    sum(normal_response_count) AS normal_response_count,
    sum(token_reported_call_count) AS token_reported_call_count,
    sum(input_token_count) AS input_token_count,
    sum(output_token_count) AS output_token_count,
    sum(total_token_count) AS total_token_count`

// QueryAIUsageSummary 查询整个时间范围的模型调用与 Token 汇总。
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
	if err := validateAIUsageMetrics(metrics); err != nil {
		return aiusage.Metrics{}, fmt.Errorf("restore AI usage summary: %w", err)
	}
	return metrics, nil
}

// QueryAIUsageTrend 按时间粒度查询模型调用与 Token 趋势。
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
	args := []any{
		query.Filter.StartTime.UnixNano(),
		query.Filter.StartTime,
		query.Filter.EndTime,
	}
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

	var previousStart time.Time
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
		if !analyticsconfig.IsSupportedTime(point.StartedAt) ||
			point.StartedAt.Before(query.Filter.StartTime) ||
			!point.StartedAt.Before(query.Filter.EndTime) ||
			!previousStart.IsZero() && !point.StartedAt.After(previousStart) {
			return nil, errors.New("stored AI usage trend contains an invalid or unordered timestamp")
		}
		if err := validateAIUsageMetrics(point.Metrics); err != nil {
			return nil, fmt.Errorf("restore AI usage trend point: %w", err)
		}
		points = append(points, point)
		previousStart = point.StartedAt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read AI usage trend rows: %w", err)
	}
	return points, nil
}

// QueryAIUsageBreakdown 按受控业务维度查询模型调用与 Token 分布。
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
	fmt.Fprintf(&statement, " AND %s != ''", dimension)
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
	seen := make(map[string]bool, query.Limit)
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
		if !validAIUsageDimensionValue(query.Dimension, item.DimensionValue) ||
			seen[item.DimensionValue] {
			return nil, errors.New("stored AI usage breakdown contains an invalid or duplicate value")
		}
		seen[item.DimensionValue] = true
		if err := validateAIUsageMetrics(item.Metrics); err != nil {
			return nil, fmt.Errorf("restore AI usage breakdown item: %w", err)
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
	var expression string
	switch bucket {
	case aiusage.TimeBucketMinute:
		expression = "started_at"
	case aiusage.TimeBucketFiveMinutes:
		expression = "toStartOfInterval(started_at, INTERVAL 5 MINUTE)"
	case aiusage.TimeBucketHour:
		expression = "toStartOfHour(started_at)"
	case aiusage.TimeBucketDay:
		expression = "toStartOfDay(started_at)"
	default:
		return "", fmt.Errorf("unsupported AI usage time bucket %d", bucket)
	}
	// 首个自然时间桶可能只覆盖查询起点之后的一部分，用实际覆盖起点作为标签。
	return "greatest(" + expression + ", fromUnixTimestamp64Nano(?, 'UTC'))", nil
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

func validateAIUsageMetrics(metrics aiusage.Metrics) error {
	if metrics.NormalResponseCount > metrics.CallCount ||
		metrics.TokenReportedCallCount > metrics.CallCount {
		return errors.New("model call counts exceed the call count")
	}
	if metrics.TotalTokens != 0 && metrics.TokenReportedCallCount == 0 {
		return errors.New("total tokens exist without a token-reported call")
	}
	return nil
}

func validAIUsageDimensionValue(dimension aiusage.Dimension, value string) bool {
	switch dimension {
	case aiusage.DimensionCaller, aiusage.DimensionRoute, aiusage.DimensionUpstream:
		return resourceconfig.IsCanonicalID(value)
	case aiusage.DimensionClientModel, aiusage.DimensionUpstreamModel:
		return routeconfig.IsValidModelName(value)
	default:
		return false
	}
}
