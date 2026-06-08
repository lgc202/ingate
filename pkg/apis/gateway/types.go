package gateway

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Kind 表示声明式资源类型
type Kind string

const (
	// KindGateway 表示 Gateway 资源类型
	KindGateway Kind = "Gateway"
	// KindRuntimeGroup 表示 RuntimeGroup 资源类型
	KindRuntimeGroup Kind = "RuntimeGroup"
	// KindRoute 表示 Route 资源类型
	KindRoute Kind = "Route"
	// KindUpstream 表示 Upstream 资源类型
	KindUpstream Kind = "Upstream"
	// KindRateLimitPolicy 表示 RateLimitPolicy 资源类型
	KindRateLimitPolicy Kind = "RateLimitPolicy"
	// KindRedisStore 表示 RedisStore 资源类型
	KindRedisStore Kind = "RedisStore"
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

// RedisMode 表示 Redis 部署模式
type RedisMode string

const (
	// RedisModeStandalone 表示单实例 Redis
	RedisModeStandalone RedisMode = "Standalone"
	// RedisModeSentinel 表示 Redis Sentinel
	RedisModeSentinel RedisMode = "Sentinel"
	// RedisModeCluster 表示 Redis Cluster
	RedisModeCluster RedisMode = "Cluster"
)

// Bundle 表示一次编译所需的资源集合
type Bundle struct {
	// +listType=atomic
	Gateways []Gateway `json:"gateways"`
	// +listType=atomic
	Routes []Route `json:"routes"`
	// +listType=atomic
	Upstreams []Upstream `json:"upstreams"`
	// +listType=atomic
	RateLimitPolicies []RateLimitPolicy `json:"rateLimitPolicies"`
	// +listType=atomic
	RedisStores []RedisStore `json:"redisStores"`
	// +listType=atomic
	PolicyBindings []PolicyBinding `json:"policyBindings"`
}

// ResourceStatus 表示声明式资源状态
type ResourceStatus struct {
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
