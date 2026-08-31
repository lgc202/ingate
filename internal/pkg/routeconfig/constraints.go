// Package routeconfig 定义 Route 各信任边界共享的稳定领域约束。
package routeconfig

import (
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
)

const (
	// DefaultRequestTimeoutMillis 是省略请求超时配置时持久化的默认值。
	DefaultRequestTimeoutMillis = 30_000
	// MinRequestTimeoutMillis 是支持的最短请求总超时。
	MinRequestTimeoutMillis = 100
	// MaxRequestTimeoutMillis 是支持的最长请求总超时。
	MaxRequestTimeoutMillis = 300_000

	// MinRetryAttempts 是支持的最小重试次数。
	MinRetryAttempts = 1
	// MaxRetryAttempts 是支持的最大重试次数。
	MaxRetryAttempts = 5
	// MinPerTryTimeoutMillis 是支持的最短单次尝试超时。
	MinPerTryTimeoutMillis = 100
	// MaxPerTryTimeoutMillis 是支持的最长单次尝试超时。
	MaxPerTryTimeoutMillis = 60_000

	// MinTargetWeight 是支持的最小转发权重。
	MinTargetWeight = 1
	// MaxTargetWeight 是支持的最大转发权重。
	MaxTargetWeight = 1_000

	// MaxGatewayRefs 限制一条 Route 可以挂载的 Gateway 数量。
	MaxGatewayRefs = 16
	// MaxHostnames 限制一条 Route 可以声明的域名数量。
	MaxHostnames = 64
	// MaxHTTPMethods 限制一条 Route 可以声明的 HTTP 方法数量。
	MaxHTTPMethods = 7
	// MaxHeaderMatches 限制一条 Route 可以声明的 Header 匹配条件数量。
	MaxHeaderMatches = 32
	// MaxHeaderModifierActions 限制一个 Header 修改器包含的动作总数。
	MaxHeaderModifierActions = 64
	// MaxPathBytes 限制 Route 匹配路径的存储大小。
	MaxPathBytes = 4 << 10
	// MaxModelNameBytes 限制客户端模型名和实际模型名的存储大小。
	MaxModelNameBytes = 256
	// MaxServiceTargets 限制一条普通 Route 的目标 Service 数量。
	MaxServiceTargets = 16
	// MaxAIModels 限制一条 AI Route 可以发布的客户端模型数量。
	MaxAIModels = 64
	// MaxAIModelTargets 限制一个客户端模型可以配置的实际模型线路数量。
	MaxAIModelTargets = 16
)

// IsValidPath 判断 value 是否为不含查询参数和片段的绝对请求路径。
func IsValidPath(value string) bool {
	if len(value) > MaxPathBytes ||
		!strings.HasPrefix(value, "/") ||
		strings.ContainsAny(value, "?#") {
		return false
	}
	_, err := url.ParseRequestURI(value)
	return err == nil
}

// IsValidModelName 判断模型名是否可以安全写入 AI 路由使用的内部 Header。
func IsValidModelName(value string) bool {
	return value != "" &&
		len(value) <= MaxModelNameBytes &&
		strings.TrimSpace(value) == value &&
		httpguts.ValidHeaderFieldValue(value)
}

// IsSupportedHTTPMethod 判断 method 是否为 Route 产品协议支持的 HTTP 方法。
func IsSupportedHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions:
		return true
	default:
		return false
	}
}

// SupportedHTTPMethods 返回 Route 产品协议支持的 HTTP 方法。
func SupportedHTTPMethods() []string {
	return []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	}
}
