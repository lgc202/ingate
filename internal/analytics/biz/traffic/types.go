package traffic

import "time"

// TimeBucket 是流量趋势查询支持的固定时间粒度
type TimeBucket uint8

const (
	TimeBucketMinute TimeBucket = iota + 1
	TimeBucketFiveMinutes
	TimeBucketHour
	TimeBucketDay
)

// Dimension 是流量分布支持的资源维度
type Dimension uint8

const (
	DimensionGateway Dimension = iota + 1
	DimensionRoute
	DimensionUpstream
)

// Filter 是流量统计的过滤条件
type Filter struct {
	StartTime  time.Time
	EndTime    time.Time
	GatewayID  string
	RouteID    string
	UpstreamID string
}

// TrendQuery 是流量趋势查询参数
type TrendQuery struct {
	Filter Filter
	Bucket TimeBucket
}

// TrendPoint 是一个时间段内的流量和延迟统计
type TrendPoint struct {
	StartedAt       time.Time
	RequestCount    uint64
	ClientErrors    uint64
	ServerErrors    uint64
	AverageDuration time.Duration
	P95Duration     time.Duration
}

// BreakdownQuery 是资源维度流量分布查询参数
type BreakdownQuery struct {
	Filter    Filter
	Dimension Dimension
	Limit     int
}

// BreakdownItem 是单个资源的流量和延迟统计
type BreakdownItem struct {
	ResourceID      string
	RequestCount    uint64
	ClientErrors    uint64
	ServerErrors    uint64
	AverageDuration time.Duration
	P95Duration     time.Duration
}
