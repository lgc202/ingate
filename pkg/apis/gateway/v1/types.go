package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ConditionType 表示声明式资源的稳定状态类型
type ConditionType string

const (
	// ConditionAccepted 表示资源当前版本是否被控制面接受
	ConditionAccepted ConditionType = "Accepted"
	// ConditionResolvedRefs 表示资源当前版本的引用是否均已解析
	ConditionResolvedRefs ConditionType = "ResolvedRefs"
	// ConditionProgrammed 表示资源当前版本是否已进入生效配置
	ConditionProgrammed ConditionType = "Programmed"
)

// ConditionReason 表示声明式资源状态变化的稳定原因
type ConditionReason string

const (
	// ReasonAccepted 表示资源已被控制面接受
	ReasonAccepted ConditionReason = "Accepted"
	// ReasonResolvedRefs 表示资源引用均已解析
	ReasonResolvedRefs ConditionReason = "ResolvedRefs"
	// ReasonProgrammed 表示资源配置已经生效
	ReasonProgrammed ConditionReason = "Programmed"
	// ReasonInvalidSpec 表示资源字段不满足要求
	ReasonInvalidSpec ConditionReason = "InvalidSpec"
	// ReasonReferenceNotFound 表示资源引用的目标不存在
	ReasonReferenceNotFound ConditionReason = "ReferenceNotFound"
	// ReasonInvalidReference 表示资源引用的目标存在但不可用
	ReasonInvalidReference ConditionReason = "InvalidReference"
	// ReasonConflict 表示资源与配置域中的其他声明冲突
	ReasonConflict ConditionReason = "Conflict"
	// ReasonUnsupported 表示当前控制面不支持该配置
	ReasonUnsupported ConditionReason = "Unsupported"
	// ReasonCompileFailed 表示资源未能生成一致的网关配置
	ReasonCompileFailed ConditionReason = "CompileFailed"
	// ReasonPending 表示资源配置仍在等待生效
	ReasonPending ConditionReason = "Pending"
	// ReasonRejected 表示资源配置被网关实例拒绝
	ReasonRejected ConditionReason = "Rejected"
	// ReasonDeliveryFailed 表示资源配置发布失败
	ReasonDeliveryFailed ConditionReason = "DeliveryFailed"
)

// Kind 表示声明式资源类型
type Kind string

const (
	// KindGateway 表示 Gateway 资源类型
	KindGateway Kind = "Gateway"
	// KindRoute 表示 Route 资源类型
	KindRoute Kind = "Route"
	// KindUpstream 表示 Upstream 资源类型
	KindUpstream Kind = "Upstream"
	// KindCertificate 表示 Certificate 资源类型
	KindCertificate Kind = "Certificate"
	// KindRateLimitPolicy 表示 RateLimitPolicy 资源类型
	KindRateLimitPolicy Kind = "RateLimitPolicy"
	// KindAccessControlPolicy 表示 AccessControlPolicy 资源类型
	KindAccessControlPolicy Kind = "AccessControlPolicy"
	// KindPolicyBinding 表示 PolicyBinding 资源类型
	KindPolicyBinding Kind = "PolicyBinding"
)

// RateLimitMode 表示限流计数状态的存放位置
type RateLimitMode string

const (
	// RateLimitModeLocal 表示每个数据面实例独立计数
	RateLimitModeLocal RateLimitMode = "Local"
	// RateLimitModeGlobal 表示通过 Redis 共享计数
	RateLimitModeGlobal RateLimitMode = "Global"
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
	// RateLimitKeyTypeConsumer 表示按认证后的 consumer 生成限流 key
	RateLimitKeyTypeConsumer RateLimitKeyType = "Consumer"
	// RateLimitKeyTypeRoute 表示按 Route 生成限流 key
	RateLimitKeyTypeRoute RateLimitKeyType = "Route"
	// RateLimitKeyTypeGateway 表示按 Gateway 生成限流 key
	RateLimitKeyTypeGateway RateLimitKeyType = "Gateway"
	// RateLimitKeyTypeRouteRule 表示按 RouteRule 生成限流 key
	RateLimitKeyTypeRouteRule RateLimitKeyType = "RouteRule"
	// RateLimitKeyTypeJWTClaim 表示按 JWT claim 生成限流 key
	RateLimitKeyTypeJWTClaim RateLimitKeyType = "JWTClaim"
	// RateLimitKeyTypeAPIKey 表示按 API Key 生成限流 key
	RateLimitKeyTypeAPIKey RateLimitKeyType = "APIKey"
	// RateLimitKeyTypeTenant 表示按租户生成限流 key
	RateLimitKeyTypeTenant RateLimitKeyType = "Tenant"
)

// RateLimitAlgorithm 表示限流计数算法
type RateLimitAlgorithm string

const (
	// RateLimitAlgorithmFixedWindow 表示固定窗口限流
	RateLimitAlgorithmFixedWindow RateLimitAlgorithm = "FixedWindow"
	// RateLimitAlgorithmSlidingWindow 表示滑动窗口限流
	RateLimitAlgorithmSlidingWindow RateLimitAlgorithm = "SlidingWindow"
	// RateLimitAlgorithmTokenBucket 表示令牌桶限流
	RateLimitAlgorithmTokenBucket RateLimitAlgorithm = "TokenBucket"
)

// RateLimitFailurePolicy 表示限流执行异常时的处理方式
type RateLimitFailurePolicy string

const (
	// RateLimitFailurePolicyFailOpen 表示限流执行失败时放行请求
	RateLimitFailurePolicyFailOpen RateLimitFailurePolicy = "FailOpen"
	// RateLimitFailurePolicyFailClose 表示限流执行失败时拒绝请求
	RateLimitFailurePolicyFailClose RateLimitFailurePolicy = "FailClose"
)

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
