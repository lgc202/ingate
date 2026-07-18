// Package wasm 提供多个内置插件共享的 Proxy-Wasm 辅助能力
package wasm

import (
	"net/url"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
)

// RouteIdentity 表示 xDS route name 中编码的 Ingate 路由身份
type RouteIdentity struct {
	GatewayName string
	RouteName   string
	RuleName    string
}

// CurrentRouteIdentity 读取当前请求命中的 xDS route name 并解析 Ingate 路由身份
func CurrentRouteIdentity(prefix string) (RouteIdentity, bool) {
	rawRouteName, ok := currentRouteName()
	if !ok {
		return RouteIdentity{}, false
	}
	return ParseRouteName(prefix, rawRouteName)
}

// CurrentRouteConfigIdentity 读取带执行配置标记和版本的 xDS route name
func CurrentRouteConfigIdentity(prefix, marker string) (RouteIdentity, string, bool) {
	rawRouteName, ok := currentRouteName()
	if !ok {
		return RouteIdentity{}, "", false
	}
	return ParseRouteConfigName(prefix, marker, rawRouteName)
}

func currentRouteName() (string, bool) {
	rawRouteName, err := proxywasm.GetProperty([]string{"xds", "route_name"})
	if err != nil || len(rawRouteName) == 0 {
		rawRouteName, err = proxywasm.GetProperty([]string{"route_name"})
	}
	if err != nil || len(rawRouteName) == 0 {
		return "", false
	}
	return string(rawRouteName), true
}

// ParseRouteName 解析 Envoy Config Compiler 生成的 route name
func ParseRouteName(prefix, value string) (RouteIdentity, bool) {
	parts := strings.Split(value, "/")
	if len(parts) < 4 || parts[0] != prefix {
		return RouteIdentity{}, false
	}

	gatewayName, err := url.PathUnescape(parts[1])
	if err != nil {
		return RouteIdentity{}, false
	}
	routeName, err := url.PathUnescape(parts[2])
	if err != nil {
		return RouteIdentity{}, false
	}
	ruleName, err := url.PathUnescape(parts[3])
	if err != nil {
		return RouteIdentity{}, false
	}

	return RouteIdentity{
		GatewayName: gatewayName,
		RouteName:   routeName,
		RuleName:    ruleName,
	}, true
}

// ParseRouteConfigName 解析带 marker/configID 后缀的 Envoy route name
func ParseRouteConfigName(prefix, marker, value string) (RouteIdentity, string, bool) {
	parts := strings.Split(value, "/")
	if len(parts) != 7 || parts[5] != marker || parts[6] == "" {
		return RouteIdentity{}, "", false
	}
	identity, ok := ParseRouteName(prefix, value)
	if !ok {
		return RouteIdentity{}, "", false
	}
	return identity, parts[6], true
}
