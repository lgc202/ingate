// Package tool 定义模型 Agent 可调用的 Ingate 原子查询工具。
package tool

import (
	"context"
	"errors"
	"time"
)

// ErrQueryTargetNotFound 表示模型引用的精确查询目标已经删除或超出保留期。
// 这不是依赖服务故障：工具应把它作为可修正结果交还模型，由模型重新获取当前列表。
var ErrQueryTargetNotFound = errors.New("assistant query target not found")

// ResourceListQuery 是配置资源列表工具共用的查询条件。
type ResourceListQuery struct {
	Text  string
	Limit int32
}

// GatewayPage 是 Agent 判断结果是否完整所需的网关分页结果。
type GatewayPage struct {
	Items   []Gateway
	HasMore bool
}

// Gateway 是运维诊断需要的网关入口信息，不包含版本等写操作字段。
type Gateway struct {
	ID        string
	Name      string
	Enabled   bool
	State     string
	Message   string
	Listeners []Listener
}

// Listener 描述网关对外暴露的一个监听入口。
type Listener struct {
	Name     string
	Protocol string
	Port     uint32
	Hostname string
}

// RoutePage 是 Agent 判断结果是否完整所需的路由分页结果。
type RoutePage struct {
	Items   []Route
	HasMore bool
}

// Route 保留诊断路由匹配和转发关系所需的最小信息。
type Route struct {
	ID            string
	Name          string
	Type          string
	Enabled       bool
	State         string
	Message       string
	AccessMode    string
	GatewayIDs    []string
	Path          string
	ServiceIDs    []string
	ExposedModels []string
}

// RouteTarget 描述路由发往一个服务的流量份额。
// Model 仅在 AI 路由中存在，表示实际发给模型服务的模型名。
type RouteTarget struct {
	ServiceID    string
	ExposedModel string
	Model        string
	Weight       uint32
}

// RouteConfiguration 保留排查一条转发链路所需的配置事实。
// 关联资源在 data 层一次解析完成，避免模型围绕 UUID 反复调用列表工具。
type RouteConfiguration struct {
	Route             Route
	Hostnames         []string
	PathMatchType     string
	Methods           []string
	Targets           []RouteTarget
	RequestTimeout    time.Duration
	RetryAttempts     uint32
	PerTryTimeout     time.Duration
	HostRewriteMode   string
	HostRewriteTarget string
	Gateways          []Gateway
	Services          []Service
}

// ServicePage 是 Agent 判断结果是否完整所需的服务分页结果。
type ServicePage struct {
	Items   []Service
	HasMore bool
}

// Service 保留诊断目标服务类型和可用状态所需的最小信息。
type Service struct {
	ID            string
	Name          string
	Type          string
	State         string
	Message       string
	EndpointCount int
	TLS           bool
	ModelProtocol string
}

// TrafficDimension 是流量统计支持的资源分组维度。
type TrafficDimension string

const (
	TrafficDimensionGateway TrafficDimension = "gateway"
	TrafficDimensionRoute   TrafficDimension = "route"
	TrafficDimensionService TrafficDimension = "service"
)

// TrafficOrder 是流量排名支持的排序依据。
type TrafficOrder string

const (
	TrafficOrderRequestCount    TrafficOrder = "request_count"
	TrafficOrderServerErrorRate TrafficOrder = "server_error_rate"
	TrafficOrderP95Duration     TrafficOrder = "p95_duration"
)

// TrafficQuery 描述一次流量分析的时间、资源范围和排名方式。
type TrafficQuery struct {
	StartTime time.Time
	EndTime   time.Time
	ScopeType string
	ScopeID   string
	GroupBy   TrafficDimension
	OrderBy   TrafficOrder
	Limit     uint32
}

// TrafficMetrics 是运维 Agent 可解释的请求量、错误和耗时摘要。
type TrafficMetrics struct {
	RequestCount     uint64
	NonErrorCount    uint64
	ClientErrorCount uint64
	ServerErrorCount uint64
	NoResponseCount  uint64
	AverageDuration  time.Duration
	P50Duration      time.Duration
	P95Duration      time.Duration
	P99Duration      time.Duration
}

// ResourceTrafficMetrics 是排名中一个资源的名称和流量指标。
type ResourceTrafficMetrics struct {
	ID      string
	Name    string
	Metrics TrafficMetrics
}

// TrafficAnalysis 是一次查询的汇总指标和资源排名。
type TrafficAnalysis struct {
	Summary TrafficMetrics
	GroupBy TrafficDimension
	OrderBy TrafficOrder
	Items   []ResourceTrafficMetrics
}

// FailureOutcome 是失败请求工具支持的结果分类。
type FailureOutcome string

const (
	FailureOutcomeClientError FailureOutcome = "client_error"
	FailureOutcomeServerError FailureOutcome = "server_error"
	FailureOutcomeNoResponse  FailureOutcome = "no_response"
)

// FailureQuery 描述近期失败请求的时间、资源和结果范围。
type FailureQuery struct {
	StartTime time.Time
	EndTime   time.Time
	ScopeType string
	ScopeID   string
	Outcome   FailureOutcome
	Limit     int32
}

// FailurePage 是近期失败请求的分页结果。
type FailurePage struct {
	ScopeName string
	Items     []Failure
	HasMore   bool
}

// Failure 只包含模型进行排障判断所需的请求元数据。
type Failure struct {
	RecordID   string
	StartedAt  time.Time
	Method     string
	Host       string
	Path       string
	StatusCode uint32
	Duration   time.Duration
	GatewayID  string
	RouteID    string
	ServiceID  string
}

// RequestRecord 是一次请求的排障元数据，不包含请求内容、凭据和内部服务地址。
type RequestRecord struct {
	RecordID         string
	StartedAt        time.Time
	Duration         time.Duration
	TimeToFirstByte  *time.Duration
	Method           string
	Host             string
	Path             string
	StatusCode       uint32
	Outcome          string
	RequestBytes     uint64
	ResponseBytes    uint64
	GatewayID        string
	RouteID          string
	ServiceID        string
	Protocol         string
	RejectionReason  string
	UpstreamAttempts uint32
	AIModelCall      *AIModelCall
	CallerID         string
}

// CallerTokenQuota 汇总一个调用方当前实际执行的 Token 额度。
// Usages 为空表示没有生效的额度限制，而不是额度服务查询失败。
type CallerTokenQuota struct {
	CallerID   string
	CallerName string
	Enabled    bool
	Usages     []TokenQuotaUsage
}

// TokenQuotaUsage 是一个额度周期内已经结算的 Token 用量和重置时间。
type TokenQuotaUsage struct {
	PolicyID        string
	PolicyName      string
	Period          string
	UsedTokens      int64
	LimitTokens     int64
	RemainingTokens int64
	StartedAt       time.Time
	ResetsAt        time.Time
}

// AIModelCall 是一次 AI 请求由模型服务返回的调用结果。
type AIModelCall struct {
	ClientModel   string
	UpstreamModel string
	Protocol      string
	ResponseModel string
	FinishReason  string
	InputTokens   *uint64
	OutputTokens  *uint64
	TotalTokens   *uint64
}

// GatewayReader 是网关列表工具所需的最小查询边界。
type GatewayReader interface {
	ListGateways(ctx context.Context, query ResourceListQuery) (GatewayPage, error)
}

// RouteReader 是路由列表工具所需的最小查询边界。
type RouteReader interface {
	ListRoutes(ctx context.Context, query ResourceListQuery) (RoutePage, error)
}

// RouteConfigurationReader 是单条路由配置链路工具所需的查询边界。
type RouteConfigurationReader interface {
	GetRouteConfiguration(ctx context.Context, routeID string) (RouteConfiguration, error)
}

// ServiceReader 是服务列表工具所需的最小查询边界。
type ServiceReader interface {
	ListServices(ctx context.Context, query ResourceListQuery) (ServicePage, error)
}

// TrafficReader 是流量分析工具所需的查询边界。
type TrafficReader interface {
	AnalyzeTraffic(ctx context.Context, query TrafficQuery) (TrafficAnalysis, error)
}

// FailureReader 是失败请求工具所需的查询边界。
type FailureReader interface {
	ListFailures(ctx context.Context, query FailureQuery) (FailurePage, error)
}

// RequestRecordReader 是单次请求明细工具所需的查询边界。
type RequestRecordReader interface {
	GetRequestRecord(ctx context.Context, recordID string, startedAt time.Time) (RequestRecord, error)
}

// CallerTokenQuotaReader 是单个调用方额度诊断工具所需的查询边界。
type CallerTokenQuotaReader interface {
	GetCallerTokenQuota(ctx context.Context, callerID string) (CallerTokenQuota, error)
}

// QuerySource 明确列出运维 Agent 当前需要的所有外部查询能力。
// 单个工具只接收自己的窄接口；这里仅作为进程装配点组合这些能力。
type QuerySource interface {
	GatewayReader
	RouteReader
	RouteConfigurationReader
	ServiceReader
	TrafficReader
	FailureReader
	RequestRecordReader
	CallerTokenQuotaReader
}
