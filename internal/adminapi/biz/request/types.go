// Package request 定义控制台请求记录查询的业务契约
package request

import "time"

// Outcome 是按 HTTP 状态码归纳的请求结果
type Outcome uint8

const (
	// OutcomeUnknown 表示没有可归类的 HTTP 状态码
	OutcomeUnknown Outcome = iota
	// OutcomeSuccess 表示状态码小于 400
	OutcomeSuccess
	// OutcomeClientError 表示状态码位于 400 到 499
	OutcomeClientError
	// OutcomeServerError 表示状态码不小于 500
	OutcomeServerError
)

// Record 是控制台排障使用的单次请求元数据
type Record struct {
	ID                  string
	RequestID           string
	StartedAt           time.Time
	Duration            *time.Duration
	TimeToFirstByte     *time.Duration
	ClientIP            string
	Method              string
	Host                string
	Path                string
	StatusCode          uint32
	Outcome             Outcome
	RequestBytes        uint64
	ResponseBytes       uint64
	GatewayID           string
	RouteID             string
	ServiceID           string
	Protocol            string
	ResponseCodeDetails string
	UpstreamAttempts    uint32
	UpstreamAddress     string
	ProxyInstanceID     string
}

// Summary 是请求记录列表展示所需的最小字段集
type Summary struct {
	ID         string
	StartedAt  time.Time
	Duration   *time.Duration
	Method     string
	Host       string
	Path       string
	StatusCode uint32
	Outcome    Outcome
	GatewayID  string
	RouteID    string
	ServiceID  string
}

// Filter 是请求记录的结构化过滤条件，时间范围为左闭右开
type Filter struct {
	StartTime  time.Time
	EndTime    time.Time
	GatewayID  string
	RouteID    string
	ServiceID  string
	RequestID  string
	Method     string
	Host       string
	PathPrefix string
	Outcome    Outcome
	StatusCode *uint16
}

// ListOptions 是请求记录分页查询参数
type ListOptions struct {
	Filter    Filter
	PageSize  int
	PageToken string
}

// Page 是按请求开始时间倒序排列的请求记录分页结果
type Page struct {
	Records       []Summary
	NextPageToken string
}
