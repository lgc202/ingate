package aiusage

import "time"

// TimeBucket 是模型用量趋势查询使用的时间粒度
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

// Dimension 是模型用量排名支持的业务维度
type Dimension uint8

const (
	// DimensionCaller 按调用方排名
	DimensionCaller Dimension = iota + 1
	// DimensionRoute 按 AI 路由排名
	DimensionRoute
	// DimensionClientModel 按客户端模型名排名
	DimensionClientModel
	// DimensionService 按实际承载调用的模型服务排名
	DimensionService
	// DimensionActualModel 按模型服务使用的实际模型名排名
	DimensionActualModel
)

// BreakdownOrder 是模型用量排名的排序依据
type BreakdownOrder uint8

const (
	// BreakdownOrderCallCount 按模型调用次数从高到低排序
	BreakdownOrderCallCount BreakdownOrder = iota + 1
	// BreakdownOrderTotalTokens 按总 Token 数从高到低排序
	BreakdownOrderTotalTokens
)

// Filter 是模型用量分析的时间与业务范围
type Filter struct {
	StartTime   time.Time
	EndTime     time.Time
	GatewayID   string
	CallerID    string
	RouteID     string
	ClientModel string
	ServiceID   string
	ActualModel string
}

// Query 是一次模型用量分析查询
type Query struct {
	Filter    Filter
	Bucket    TimeBucket
	Dimension Dimension
	Order     BreakdownOrder
	Limit     int
}

// Metrics 是一个范围内的模型调用与 Token 原始计数
type Metrics struct {
	CallCount              uint64
	NormalResponseCount    uint64
	TokenReportedCallCount uint64
	InputTokens            uint64
	OutputTokens           uint64
	TotalTokens            uint64
}

// TrendPoint 是一个时间段内的模型用量统计
type TrendPoint struct {
	StartedAt time.Time
	Metrics   Metrics
}

// BreakdownItem 是一个业务维度值的模型用量统计
type BreakdownItem struct {
	DimensionValue string
	Metrics        Metrics
}

// Analysis 是控制台一次展示所需的汇总、趋势与排名
type Analysis struct {
	Summary   Metrics
	Trend     []TrendPoint
	Dimension Dimension
	Order     BreakdownOrder
	Breakdown []BreakdownItem
}
