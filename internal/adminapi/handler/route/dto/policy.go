package dto

// RoutePolicySource 表示控制台策略绑定来自哪类正式资源模型
type RoutePolicySource string

const (
	// routePolicySourceNative 表示绑定会直接落入 RouteSpec 的原生字段
	routePolicySourceNative RoutePolicySource = "RouteNative"
)

// RoutePolicyCapability 表示路由级能力的稳定产品标识，不能使用展示文案作为主键
type RoutePolicyCapability string

const (
	routePolicyRequestHeaderModifier RoutePolicyCapability = "RequestHeaderModifier"
	routePolicyTimeout               RoutePolicyCapability = "Timeout"
	routePolicyRetry                 RoutePolicyCapability = "Retry"
)

const (
	paramSetHeadersOn        = "setHeadersOn"
	paramHeaderValue         = "value"
	paramRemoveHeadersOn     = "removeHeadersOn"
	paramTimeoutMillis       = "timeoutMillis"
	paramRetryAttempts       = "attempts"
	paramPerTryTimeoutMillis = "perTryTimeoutMillis"
	minRouteTimeoutMillis    = 100
	maxRouteTimeoutMillis    = 300000
	minRetryAttempts         = 1
	maxRetryAttempts         = 5
	minPerTryTimeoutMillis   = 100
	maxPerTryTimeoutMillis   = 60000
)
