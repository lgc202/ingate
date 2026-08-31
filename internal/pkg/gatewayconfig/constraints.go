// Package gatewayconfig 定义 Gateway 各信任边界共享的稳定领域约束。
package gatewayconfig

import utilvalidation "k8s.io/apimachinery/pkg/util/validation"

const (
	// MaxListeners 限制一个 Gateway 可以声明的 Listener 数量。
	MaxListeners = 64
	// MinListenerPort 是 Listener 可以使用的最小 TCP 端口。
	MinListenerPort = 1
	// MaxListenerPort 是 Listener 可以使用的最大 TCP 端口。
	MaxListenerPort = 65_535
)

// IsValidListenerName 判断 name 是否为合法的 Listener 局部标识。
func IsValidListenerName(name string) bool {
	return len(utilvalidation.IsDNS1123Label(name)) == 0
}

// IsValidListenerPort 判断 port 是否处于 Listener 支持的 TCP 端口范围内。
func IsValidListenerPort(port int) bool {
	return port >= MinListenerPort && port <= MaxListenerPort
}
