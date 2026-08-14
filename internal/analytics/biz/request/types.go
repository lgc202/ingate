package request

import (
	"time"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
)

// StatusClass 是根据 HTTP 状态码派生的请求结果分类
type StatusClass uint8

const (
	// StatusClassUnknown 表示请求没有可识别的 HTTP 结果
	StatusClassUnknown StatusClass = iota
	// StatusClassSuccess 表示状态码位于 1xx 到 3xx
	StatusClassSuccess
	// StatusClassClientError 表示状态码位于 4xx
	StatusClassClientError
	// StatusClassServerError 表示状态码位于 5xx 及以上
	StatusClassServerError
)

// Fact 保存 ALS 原始请求元数据和 Analytics 派生字段
type Fact struct {
	Record      *alsv1.RequestRecord
	StatusClass StatusClass
}

// Filter 是请求明细查询的结构化过滤条件，时间范围为左闭右开
type Filter struct {
	StartTime   time.Time
	EndTime     time.Time
	GatewayID   string
	RouteID     string
	UpstreamID  string
	RequestID   string
	Method      string
	Host        string
	PathPrefix  string
	StatusClass StatusClass
	StatusCode  *uint16
}

// Cursor 标识时间倒序分页中最后一条请求的位置
type Cursor struct {
	StartedAt time.Time
	ID        string
}

// ListOptions 是请求明细分页查询参数
type ListOptions struct {
	Filter   Filter
	PageSize int
	Cursor   *Cursor
}

// Page 是按 started_at 和 id 倒序排列的请求明细分页结果
type Page struct {
	Records    []*alsv1.RequestRecord
	NextCursor *Cursor
}
