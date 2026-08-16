package traffic

import "time"

// TimeBucket 是趋势查询使用的时间粒度
type TimeBucket uint8

const (
	// TimeBucketMinute 表示每分钟一个趋势点
	TimeBucketMinute TimeBucket = iota + 1
	// TimeBucketFiveMinutes 表示每五分钟一个趋势点
	TimeBucketFiveMinutes
	// TimeBucketHour 表示每小时一个趋势点
	TimeBucketHour
	// TimeBucketDay 表示每天一个趋势点
	TimeBucketDay
)

// Dimension 是流量排名支持的资源维度
type Dimension uint8

const (
	// DimensionGateway 按接收请求的网关排名
	DimensionGateway Dimension = iota + 1
	// DimensionRoute 按命中的路由排名
	DimensionRoute
	// DimensionService 按最终转发到的服务排名
	DimensionService
)

// BreakdownOrder 是资源流量排名的排序依据
type BreakdownOrder uint8

const (
	// BreakdownOrderRequestCount 按请求量从高到低排序
	BreakdownOrderRequestCount BreakdownOrder = iota + 1
	// BreakdownOrderServerErrorRate 按服务端错误率从高到低排序
	BreakdownOrderServerErrorRate
	// BreakdownOrderP95Duration 按 P95 总耗时从高到低排序
	BreakdownOrderP95Duration
)

// Filter 是流量分析的时间与资源范围
type Filter struct {
	StartTime time.Time
	EndTime   time.Time
	GatewayID string
	RouteID   string
	ServiceID string
}

// Query 是一次流量分析查询
type Query struct {
	Filter    Filter
	Bucket    TimeBucket
	Dimension Dimension
	Order     BreakdownOrder
	Limit     int
}

// Metrics 是一个范围内的流量与延迟统计
type Metrics struct {
	RequestCount    uint64
	NonErrorCount   uint64
	ClientErrors    uint64
	ServerErrors    uint64
	NoResponses     uint64
	AverageDuration time.Duration
	P50Duration     time.Duration
	P95Duration     time.Duration
	P99Duration     time.Duration
}

// TrendPoint 是一个时间段内的流量统计
type TrendPoint struct {
	StartedAt time.Time
	Metrics   Metrics
}

// BreakdownItem 是单个资源的流量统计
type BreakdownItem struct {
	ResourceID string
	Metrics    Metrics
}

// Analysis 是控制台一次展示所需的汇总、趋势和资源排名
type Analysis struct {
	Summary   Metrics
	Trend     []TrendPoint
	Dimension Dimension
	Order     BreakdownOrder
	Breakdown []BreakdownItem
}
