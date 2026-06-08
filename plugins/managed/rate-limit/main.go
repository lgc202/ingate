package main

import (
	"net/url"
	"strings"

	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/config"
	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/ratelimit"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const (
	cookieHeaderName   = "cookie"
	consumerHeaderName = "x-ingate-consumer"
	routeNamePrefix    = "ingate-route"
)

type pluginContext struct {
	types.DefaultPluginContext

	config config.PluginConfig
}

type httpContext struct {
	types.DefaultHttpContext

	plugin       *pluginContext
	quotaHeaders map[string]string
}

type routeIdentity struct {
	GatewayName string
	RouteName   string
	RuleName    string
}

var localLimiter = ratelimit.NewSharedDataLocalLimiter()

func main() {}

func init() {
	proxywasm.SetPluginContext(func(contextID uint32) types.PluginContext {
		return &pluginContext{}
	})
}

func (p *pluginContext) OnPluginStart(pluginConfigurationSize int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil && err != types.ErrorStatusNotFound {
		proxywasm.LogErrorf("read managed rate-limit config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}

	pluginConfig, err := config.ParsePluginConfig(data)
	if err != nil {
		proxywasm.LogErrorf("parse managed rate-limit config failed: %v", err)
		return types.OnPluginStartStatusFailed
	}
	p.config = pluginConfig
	return types.OnPluginStartStatusOK
}

func (p *pluginContext) NewHttpContext(contextID uint32) types.HttpContext {
	return &httpContext{plugin: p}
}

func (h *httpContext) OnHttpRequestHeaders(numHeaders int, endOfStream bool) types.Action {
	route, ok := h.routeConfig()
	if !ok || len(route.Bindings) == 0 {
		return types.ActionContinue
	}

	req := requestFromProxyWasm(route)
	result := localLimiter.Evaluate(route, req)
	for _, err := range result.Errors {
		proxywasm.LogErrorf("managed rate-limit local rule evaluation failed: %v", err)
	}
	if !result.Allowed {
		sendRejected(result.Decision)
		return types.ActionPause
	}
	if len(result.QuotaHeaders) > 0 {
		h.quotaHeaders = result.QuotaHeaders
	}

	for _, check := range result.RedisChecks {
		// Redis-backed global limit 需要 Ingate 自己的数据面执行器或明确的 host function。
		// 当前插件不继承 Higress wrapper 的 Redis client，避免把非标准运行时能力固化进产品链路。
		proxywasm.LogErrorf(
			"managed rate-limit global rule cannot run without redis executor: policy=%s rule=%s redisStore=%s",
			check.Policy.Name,
			check.Rule.Name,
			check.RedisStore,
		)
		if check.Policy.FailOpen() {
			continue
		}
		sendRejected(ratelimit.Decision{
			Allowed:    false,
			StatusCode: check.Policy.RejectedStatusCode(),
			Message:    check.Policy.RejectedMessage(),
			Policy:     check.Policy,
			Rule:       check.Rule,
			Key:        check.Key,
		})
		return types.ActionPause
	}

	return types.ActionContinue
}

func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	for name, value := range h.quotaHeaders {
		_ = proxywasm.ReplaceHttpResponseHeader(name, value)
	}
	return types.ActionContinue
}

func (h *httpContext) routeConfig() (config.RouteConfig, bool) {
	identity, ok := currentRouteIdentity()
	if !ok {
		return config.RouteConfig{}, false
	}

	for _, route := range h.plugin.config.Routes {
		if route.GatewayName == identity.GatewayName && route.RouteName == identity.RouteName && route.RuleName == identity.RuleName {
			return route, true
		}
	}
	return config.RouteConfig{}, false
}

func currentRouteIdentity() (routeIdentity, bool) {
	rawRouteName, err := proxywasm.GetProperty([]string{"xds", "route_name"})
	if err != nil || len(rawRouteName) == 0 {
		rawRouteName, err = proxywasm.GetProperty([]string{"route_name"})
	}
	if err != nil || len(rawRouteName) == 0 {
		return routeIdentity{}, false
	}

	return parseRouteName(string(rawRouteName))
}

func parseRouteName(value string) (routeIdentity, bool) {
	parts := strings.Split(value, "/")
	if len(parts) < 4 || parts[0] != routeNamePrefix {
		return routeIdentity{}, false
	}

	gatewayName, err := url.PathUnescape(parts[1])
	if err != nil {
		return routeIdentity{}, false
	}
	routeName, err := url.PathUnescape(parts[2])
	if err != nil {
		return routeIdentity{}, false
	}
	ruleName, err := url.PathUnescape(parts[3])
	if err != nil {
		return routeIdentity{}, false
	}

	return routeIdentity{
		GatewayName: gatewayName,
		RouteName:   routeName,
		RuleName:    ruleName,
	}, true
}

func sendRejected(decision ratelimit.Decision) {
	_ = proxywasm.SendHttpResponse(
		uint32(decision.StatusCode),
		headerPairs(decision.QuotaHeaders),
		[]byte(decision.Message),
		-1,
	)
}

func requestFromProxyWasm(route config.RouteConfig) ratelimit.Request {
	path, _ := proxywasm.GetHttpRequestHeader(":path")
	remoteAddr := sourceAddress()
	headers := make(map[string]string)
	for _, name := range headerNames(route) {
		value, err := proxywasm.GetHttpRequestHeader(name)
		if err == nil && value != "" {
			headers[name] = value
		}
	}
	return ratelimit.Request{
		GatewayName: route.GatewayName,
		RouteName:   route.RouteName,
		RuleName:    route.RuleName,
		Path:        path,
		RemoteAddr:  remoteAddr,
		Headers:     headers,
	}
}

func headerNames(route config.RouteConfig) []string {
	seen := map[string]struct{}{
		cookieHeaderName:   {},
		consumerHeaderName: {},
	}
	for _, binding := range route.Bindings {
		for _, policy := range binding.Policies {
			for _, rule := range policy.Rules {
				for _, part := range rule.Key {
					if part.Type == config.KeyTypeHeader && part.Name != "" {
						seen[part.Name] = struct{}{}
					}
					if part.Type == config.KeyTypeCookie {
						seen[cookieHeaderName] = struct{}{}
					}
					if part.Type == config.KeyTypeConsumer {
						seen[consumerHeaderName] = struct{}{}
					}
				}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

func sourceAddress() string {
	value, err := proxywasm.GetProperty([]string{"source", "address"})
	if err != nil {
		return ""
	}
	return string(value)
}

func headerPairs(headers map[string]string) [][2]string {
	if len(headers) == 0 {
		return nil
	}
	result := make([][2]string, 0, len(headers))
	for name, value := range headers {
		result = append(result, [2]string{name, value})
	}
	return result
}
