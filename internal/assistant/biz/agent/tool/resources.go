// Package tool 定义模型 Agent 可调用的 Ingate 原子查询工具。
package tool

import (
	"context"
	"time"
)

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

// TrafficQuery 描述一次聚合流量查询的时间和资源范围。
type TrafficQuery struct {
	StartTime    time.Time
	EndTime      time.Time
	ResourceType string
	ResourceID   string
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

// FailureOutcome 是失败请求工具支持的结果分类。
type FailureOutcome string

const (
	FailureOutcomeClientError FailureOutcome = "client_error"
	FailureOutcomeServerError FailureOutcome = "server_error"
	FailureOutcomeNoResponse  FailureOutcome = "no_response"
)

// FailureQuery 描述近期失败请求的时间、资源和结果范围。
type FailureQuery struct {
	StartTime    time.Time
	EndTime      time.Time
	ResourceType string
	ResourceID   string
	Outcome      FailureOutcome
	Limit        int32
}

// FailurePage 是近期失败请求的分页结果。
type FailurePage struct {
	Items   []Failure
	HasMore bool
}

// Failure 只包含模型进行排障判断所需的请求元数据。
type Failure struct {
	StartedAt  time.Time
	Method     string
	StatusCode uint32
	Duration   time.Duration
	GatewayID  string
	RouteID    string
	ServiceID  string
}

// GatewayReader 是网关列表工具所需的最小查询边界。
type GatewayReader interface {
	ListGateways(ctx context.Context, query ResourceListQuery) (GatewayPage, error)
}

// RouteReader 是路由列表工具所需的最小查询边界。
type RouteReader interface {
	ListRoutes(ctx context.Context, query ResourceListQuery) (RoutePage, error)
}

// ServiceReader 是服务列表工具所需的最小查询边界。
type ServiceReader interface {
	ListServices(ctx context.Context, query ResourceListQuery) (ServicePage, error)
}

// TrafficReader 是聚合流量工具所需的查询边界。
type TrafficReader interface {
	GetTraffic(ctx context.Context, query TrafficQuery) (TrafficMetrics, error)
}

// FailureReader 是失败请求工具所需的查询边界。
type FailureReader interface {
	ListFailures(ctx context.Context, query FailureQuery) (FailurePage, error)
}

// QuerySource 明确列出运维 Agent 当前需要的所有外部查询能力。
// 单个工具只接收自己的窄接口；这里仅作为进程装配点组合这些能力。
type QuerySource interface {
	GatewayReader
	RouteReader
	ServiceReader
	TrafficReader
	FailureReader
}
