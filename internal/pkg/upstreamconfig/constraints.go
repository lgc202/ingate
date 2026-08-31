// Package upstreamconfig 定义 Upstream 各信任边界共享的稳定领域约束。
package upstreamconfig

import (
	"net/netip"
	"net/url"
	"strings"

	"golang.org/x/net/http/httpguts"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
)

const (
	// MaxEndpoints 限制一个 Upstream 可以声明的 Endpoint 数量。
	MaxEndpoints = 64

	// MinEndpointPort 是 Endpoint 可以使用的最小 TCP 端口。
	MinEndpointPort = 1
	// MaxEndpointPort 是 Endpoint 可以使用的最大 TCP 端口。
	MaxEndpointPort = 65_535
	// DefaultEndpointWeight 是省略 Endpoint 权重时持久化的默认值。
	DefaultEndpointWeight = 1
	// MinEndpointWeight 是 Endpoint 支持的最小相对权重。
	MinEndpointWeight = 1
	// MaxEndpointWeight 是 Endpoint 支持的最大相对权重。
	MaxEndpointWeight = 1_000

	// DefaultHealthCheckIntervalSeconds 是省略健康检查间隔时持久化的默认值。
	DefaultHealthCheckIntervalSeconds = 10
	// MinHealthCheckIntervalSeconds 是支持的最短健康检查间隔。
	MinHealthCheckIntervalSeconds = 1
	// MaxHealthCheckIntervalSeconds 是支持的最长健康检查间隔。
	MaxHealthCheckIntervalSeconds = 300
	// DefaultHealthCheckTimeoutSeconds 是省略健康检查超时时间时持久化的默认值。
	DefaultHealthCheckTimeoutSeconds = 2
	// MinHealthCheckTimeoutSeconds 是支持的最短健康检查超时时间。
	MinHealthCheckTimeoutSeconds = 1
	// MaxHealthCheckTimeoutSeconds 是支持的最长健康检查超时时间。
	MaxHealthCheckTimeoutSeconds = 60
	// MaxHealthCheckPathBytes 限制健康检查请求路径的存储大小。
	MaxHealthCheckPathBytes = 4 << 10

	// MaxModelAPIKeyBytes 限制模型服务凭据的存储大小。
	MaxModelAPIKeyBytes = 4 << 10
)

// NormalizeAddress 规范化 Endpoint 地址或 TLS 服务端名称。
func NormalizeAddress(value string) string {
	value = strings.TrimSpace(value)
	if address, err := netip.ParseAddr(value); err == nil {
		return address.String()
	}
	return strings.ToLower(value)
}

// IsValidAddress 判断 value 是否为合法的 IP 地址或 DNS 主机名。
func IsValidAddress(value string) bool {
	if _, err := netip.ParseAddr(value); err == nil {
		return true
	}
	return len(utilvalidation.IsDNS1123Subdomain(value)) == 0
}

// IsValidEndpointPort 判断 port 是否处于 Endpoint 支持的 TCP 端口范围内。
func IsValidEndpointPort(port int) bool {
	return port >= MinEndpointPort && port <= MaxEndpointPort
}

// IsValidEndpointWeight 判断 weight 是否处于 Endpoint 支持的相对权重范围内。
func IsValidEndpointWeight(weight int) bool {
	return weight >= MinEndpointWeight && weight <= MaxEndpointWeight
}

// IsValidHealthCheckPath 判断 value 是否为不含查询参数和片段的绝对请求路径。
func IsValidHealthCheckPath(value string) bool {
	if len(value) > MaxHealthCheckPathBytes ||
		!strings.HasPrefix(value, "/") ||
		strings.ContainsAny(value, "?#") {
		return false
	}
	_, err := url.ParseRequestURI(value)
	return err == nil
}

// IsValidHealthCheckInterval 判断 seconds 是否处于支持的健康检查间隔范围内。
func IsValidHealthCheckInterval(seconds int) bool {
	return seconds >= MinHealthCheckIntervalSeconds &&
		seconds <= MaxHealthCheckIntervalSeconds
}

// IsValidHealthCheckTimeout 判断 timeoutSeconds 是否有效且短于 intervalSeconds。
func IsValidHealthCheckTimeout(timeoutSeconds, intervalSeconds int) bool {
	return timeoutSeconds >= MinHealthCheckTimeoutSeconds &&
		timeoutSeconds <= MaxHealthCheckTimeoutSeconds &&
		timeoutSeconds < intervalSeconds
}

// IsValidModelAPIKey 判断 value 是否可以作为模型服务凭据持久化。
func IsValidModelAPIKey(value string) bool {
	return len(value) <= MaxModelAPIKeyBytes &&
		strings.TrimSpace(value) == value &&
		httpguts.ValidHeaderFieldValue(value)
}
