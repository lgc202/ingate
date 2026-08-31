package request

import (
	"errors"
	"time"
)

const (
	// StatusClassUnknown 表示请求没有可识别的 HTTP 结果。
	StatusClassUnknown StatusClass = iota
	// StatusClassSuccess 表示状态码位于 1xx 到 3xx。
	StatusClassSuccess
	// StatusClassClientError 表示状态码位于 4xx。
	StatusClassClientError
	// StatusClassServerError 表示状态码位于 5xx 及以上。
	StatusClassServerError
	// StatusClassNoResponse 表示请求没有获得有效 HTTP 状态码。
	StatusClassNoResponse
)

// ErrNotFound 表示请求记录不存在或已经超过明细保留期。
var ErrNotFound = errors.New("request record not found")

// StatusClass 是根据 HTTP 状态码派生的请求结果分类。
type StatusClass uint8

// ModelCall 保存 AI Route 已观测到的模型映射和 Token 用量。
// 选路前被拒绝的请求可能只有 ClientModel，不一定已经调用模型服务。
type ModelCall struct {
	ClientModel      string
	UpstreamModel    string
	UpstreamProtocol string
	ResponseModel    string
	FinishReason     string
	InputTokens      *uint64
	OutputTokens     *uint64
	TotalTokens      *uint64
}

// Record 保存一次已完成请求的排障和聚合元数据。
//
// ALS Proto 只用于 Kafka 传输，进入 biz 后转换为该领域类型，避免存储实现依赖采集协议。
type Record struct {
	ID                  string
	RequestID           string
	StartedAt           time.Time
	Duration            *time.Duration
	ClientIP            string
	Method              string
	Host                string
	Path                string
	StatusCode          uint16
	StatusClass         StatusClass
	RequestBytes        uint64
	ResponseBytes       uint64
	GatewayID           string
	RouteID             string
	UpstreamID          string
	CallerID            string
	AccessKeyID         string
	EnvoyNodeID         string
	Protocol            string
	ResponseCodeDetails string
	UpstreamAttempts    uint16
	UpstreamAddress     string
	TimeToFirstByte     *time.Duration
	ModelCall           *ModelCall
}

// Summary 保存请求列表展示和进入详情所需的最小字段集。
type Summary struct {
	ID                  string
	StartedAt           time.Time
	Duration            *time.Duration
	Method              string
	Host                string
	Path                string
	StatusCode          uint16
	GatewayID           string
	RouteID             string
	UpstreamID          string
	CallerID            string
	AccessKeyID         string
	ResponseCodeDetails string
	ModelCall           *ModelCall
}

// Filter 是请求明细查询的结构化过滤条件，时间范围为左闭右开。
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
	CallerID    string
}

// Cursor 标识时间倒序分页中最后一条请求的位置。
type Cursor struct {
	StartedAt time.Time
	ID        string
}

// ListOptions 是请求明细分页查询参数。
type ListOptions struct {
	Filter   Filter
	PageSize int
	Cursor   *Cursor
}

// Page 是按 started_at 和 id 倒序排列的请求明细分页结果。
type Page struct {
	Records    []Summary
	NextCursor *Cursor
}

// ClassifyStatusCode 根据 HTTP 状态码返回请求结果分类。
func ClassifyStatusCode(statusCode uint16) StatusClass {
	switch {
	case statusCode >= 500:
		return StatusClassServerError
	case statusCode >= 400:
		return StatusClassClientError
	case statusCode >= 100:
		return StatusClassSuccess
	default:
		return StatusClassNoResponse
	}
}
