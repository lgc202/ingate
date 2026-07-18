package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Kind 表示声明式资源类型
type Kind string

const (
	// KindGateway 表示 Gateway 资源类型
	KindGateway Kind = "Gateway"
	// KindRoute 表示 Route 资源类型
	KindRoute Kind = "Route"
	// KindUpstream 表示 Upstream 资源类型
	KindUpstream Kind = "Upstream"
	// KindUpstreamCredential 表示 UpstreamCredential 资源类型
	KindUpstreamCredential Kind = "UpstreamCredential"
	// KindCertificate 表示 Certificate 资源类型
	KindCertificate Kind = "Certificate"
	// KindRateLimitPolicy 表示 RateLimitPolicy 资源类型
	KindRateLimitPolicy Kind = "RateLimitPolicy"
	// KindAccessControlPolicy 表示 AccessControlPolicy 资源类型
	KindAccessControlPolicy Kind = "AccessControlPolicy"
)

// RateLimitKeyType 表示限流 key 的组成维度
type RateLimitKeyType string

const (
	// RateLimitKeyTypeIP 表示按客户端 IP 生成限流 key
	RateLimitKeyTypeIP RateLimitKeyType = "IP"
	// RateLimitKeyTypeHeader 表示按请求 header 生成限流 key
	RateLimitKeyTypeHeader RateLimitKeyType = "Header"
	// RateLimitKeyTypeQuery 表示按 query 参数生成限流 key
	RateLimitKeyTypeQuery RateLimitKeyType = "Query"
	// RateLimitKeyTypeCookie 表示按 cookie 生成限流 key
	RateLimitKeyTypeCookie RateLimitKeyType = "Cookie"
	// RateLimitKeyTypeRoute 表示按 Route 生成限流 key
	RateLimitKeyTypeRoute RateLimitKeyType = "Route"
	// RateLimitKeyTypeGateway 表示按 Gateway 生成限流 key
	RateLimitKeyTypeGateway RateLimitKeyType = "Gateway"
	// RateLimitKeyTypeRouteRule 表示按 RouteRule 生成限流 key
	RateLimitKeyTypeRouteRule RateLimitKeyType = "RouteRule"
)

// RateLimitFailurePolicy 表示限流执行异常时的处理方式
type RateLimitFailurePolicy string

const (
	// RateLimitFailurePolicyFailOpen 表示限流执行失败时放行请求
	RateLimitFailurePolicyFailOpen RateLimitFailurePolicy = "FailOpen"
	// RateLimitFailurePolicyFailClose 表示限流执行失败时拒绝请求
	RateLimitFailurePolicyFailClose RateLimitFailurePolicy = "FailClose"
)

// PolicyTargetRef 表示策略的生效目标
type PolicyTargetRef struct {
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
}

// PolicyStatus 表示策略的总体状态和各目标生效状态
type PolicyStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +listType=atomic
	Targets []PolicyTargetStatus `json:"targets,omitempty"`
}

// PolicyTargetStatus 表示策略在单个目标上的生效状态
type PolicyTargetStatus struct {
	TargetRef PolicyTargetRef `json:"targetRef"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
