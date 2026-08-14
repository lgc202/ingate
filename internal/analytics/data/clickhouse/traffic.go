package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lgc202/ingate/internal/analytics/biz/traffic"
)

const trafficAggregates = `
    sum(request_count) AS request_count,
    sum(client_error_count) AS client_error_count,
    sum(server_error_count) AS server_error_count,
    toUInt64(round(coalesce(avgMerge(duration_average), 0))) AS average_duration_ns,
    toUInt64(round(coalesce(quantileTDigestMerge(0.95)(duration_p95), 0))) AS p95_duration_ns`

// QueryTrafficTrend 从分钟聚合状态查询流量和延迟趋势
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
			p95DurationNS     uint64
		)
		if err := rows.Scan(
			&point.StartedAt,
			&point.RequestCount,
			&point.ClientErrors,
			&point.ServerErrors,
			&averageDurationNS,
			&p95DurationNS,
		); err != nil {
			return nil, fmt.Errorf("scan traffic trend: %w", err)
		}
		point.AverageDuration = time.Duration(averageDurationNS)
		point.P95Duration = time.Duration(p95DurationNS)
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read traffic trend rows: %w", err)
	}
	return points, nil
}

// QueryTrafficBreakdown 从分钟聚合状态查询资源维度的流量和延迟分布
func (s *Store) QueryTrafficBreakdown(
	ctx context.Context,
	query traffic.BreakdownQuery,
) (items []traffic.BreakdownItem, err error) {
	dimension, err := trafficDimensionColumn(query.Dimension)
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
	statement.WriteString(" GROUP BY resource_id ORDER BY request_count DESC, resource_id LIMIT ?")
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
			p95DurationNS     uint64
		)
		if err := rows.Scan(
			&item.ResourceID,
			&item.RequestCount,
			&item.ClientErrors,
			&item.ServerErrors,
			&averageDurationNS,
			&p95DurationNS,
		); err != nil {
			return nil, fmt.Errorf("scan traffic breakdown: %w", err)
		}
		item.AverageDuration = time.Duration(averageDurationNS)
		item.P95Duration = time.Duration(p95DurationNS)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read traffic breakdown rows: %w", err)
	}
	return items, nil
}

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
