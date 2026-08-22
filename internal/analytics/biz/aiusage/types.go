package aiusage

import "time"

// TimeBucket 是模型用量趋势查询支持的固定时间粒度
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

// Dimension 是模型用量分布支持的业务维度
type Dimension uint8

const (
	// DimensionCaller 按调用方分组
	DimensionCaller Dimension = iota + 1
	// DimensionRoute 按 AI Route 分组
	DimensionRoute
	// DimensionClientModel 按调用方使用的稳定模型名分组
	DimensionClientModel
	// DimensionUpstream 按实际承载调用的 Upstream 分组
	DimensionUpstream
	// DimensionUpstreamModel 按线路配置的真实模型名分组
	DimensionUpstreamModel
)

// BreakdownOrder 是模型用量分布的排序依据
type BreakdownOrder uint8

const (
	// BreakdownOrderCallCount 按模型调用次数从高到低排序
	BreakdownOrderCallCount BreakdownOrder = iota + 1
	// BreakdownOrderTotalTokens 按总 Token 数从高到低排序
	BreakdownOrderTotalTokens
)

// Filter 是模型用量分析的时间和业务范围
type Filter struct {
	StartTime     time.Time
	EndTime       time.Time
	GatewayID     string
	CallerID      string
	RouteID       string
	ClientModel   string
	UpstreamID    string
	UpstreamModel string
}

// Metrics 是一个范围内的模型调用和 Token 原始计数
type Metrics struct {
	CallCount              uint64
	NormalResponseCount    uint64
	TokenReportedCallCount uint64
	InputTokens            uint64
	OutputTokens           uint64
	TotalTokens            uint64
}

// TrendQuery 是模型用量趋势查询参数
type TrendQuery struct {
	Filter Filter
	Bucket TimeBucket
}

// TrendPoint 是一个时间段内的模型用量统计
type TrendPoint struct {
	StartedAt time.Time
	Metrics   Metrics
}

// TrendResult 汇总一个时间范围的整体指标及其分桶趋势
type TrendResult struct {
	Summary Metrics
	Points  []TrendPoint
}

// BreakdownQuery 是模型用量分布查询参数
type BreakdownQuery struct {
	Filter    Filter
	Dimension Dimension
	Order     BreakdownOrder
	Limit     int
}

// BreakdownItem 是一个业务维度值的模型用量统计
type BreakdownItem struct {
	DimensionValue string
	Metrics        Metrics
}
