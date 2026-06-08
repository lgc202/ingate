package main

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/config"
	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/executor"
	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/ratelimit"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const (
	aiModelHeaderName            = "x-ingate-ai-model"
	apiKeyHeaderName             = "x-ingate-api-key"
	cookieHeaderName             = "cookie"
	consumerHeaderName           = "x-ingate-consumer"
	jwtClaimHeaderPrefix         = "x-ingate-jwt-claim-"
	routeNamePrefix              = "ingate-route"
	tenantHeaderName             = "x-ingate-tenant"
	defaultExecutorTimeoutMillis = 50
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

	if len(result.RedisChecks) > 0 {
		return h.dispatchRedisChecks(result.RedisChecks)
	}

	return types.ActionContinue
}

func (h *httpContext) OnHttpResponseHeaders(numHeaders int, endOfStream bool) types.Action {
	for name, value := range h.quotaHeaders {
		_ = proxywasm.ReplaceHttpResponseHeader(name, value)
	}
	return types.ActionContinue
}

func (h *httpContext) dispatchRedisChecks(checks []ratelimit.RedisCheck) types.Action {
	executorConfig := h.plugin.config.Executor
	if executorConfig == nil || executorConfig.ClusterName == "" || executorConfig.Path == "" {
		return h.handleExecutorFailure(checks, "managed rate-limit executor is not configured")
	}

	request := executor.CheckRequest{
		SchemaVersion: executor.SchemaVersionV1,
		Checks:        make([]executor.Check, 0, len(checks)),
	}
	for _, check := range checks {
		store, ok := h.redisStore(check.RedisStore)
		if !ok {
			return h.handleExecutorFailure(checks, "managed rate-limit redis store is not configured")
		}
		request.Checks = append(request.Checks, executor.Check{
			PolicyName: check.Policy.Name,
			RuleName:   check.Rule.Name,
			RedisKey:   check.RedisKey,
			RedisStore: executor.RedisStore{
				ID:                   store.Name,
				Mode:                 store.Mode,
				Address:              store.Address,
				Addresses:            store.Addresses,
				DB:                   store.DB,
				TLS:                  store.TLS,
				TLSServerName:        store.TLSServerName,
				Username:             store.Username,
				PasswordRef:          store.PasswordRef,
				ConnectTimeoutMillis: store.ConnectTimeoutMillis,
				CommandTimeoutMillis: store.CommandTimeoutMillis,
				PoolSize:             store.PoolSize,
				MinIdleConns:         store.MinIdleConns,
				SentinelMaster:       store.SentinelMaster,
			},
			Algorithm: check.Rule.Algorithm,
			Limit: executor.Limit{
				Requests:      check.Requests,
				WindowSeconds: check.WindowSeconds,
				Burst:         check.Burst,
			},
			TimeoutMillis: check.RedisTimeoutMs,
		})
	}

	body, err := json.Marshal(request)
	if err != nil {
		proxywasm.LogErrorf("marshal managed rate-limit executor request failed: %v", err)
		return h.handleExecutorFailure(checks, "marshal executor request failed")
	}

	_, err = proxywasm.DispatchHttpCall(
		executorConfig.ClusterName,
		[][2]string{
			{":method", "POST"},
			{":path", executorConfig.Path},
			{":authority", executorConfig.ClusterName},
			{"content-type", "application/json"},
		},
		body,
		nil,
		uint32(executorTimeoutMillis(executorConfig, checks)),
		func(numHeaders, bodySize, numTrailers int) {
			h.handleExecutorResponse(checks, bodySize)
		},
	)
	if err != nil {
		proxywasm.LogErrorf("dispatch managed rate-limit executor request failed: %v", err)
		return h.handleExecutorFailure(checks, "dispatch executor request failed")
	}
	return types.ActionPause
}

func (h *httpContext) handleExecutorResponse(checks []ratelimit.RedisCheck, bodySize int) {
	status := httpCallStatus()
	if status != 200 {
		h.resumeAfterExecutorFailure(checks, "executor returned non-200 response")
		return
	}

	body, err := proxywasm.GetHttpCallResponseBody(0, bodySize)
	if err != nil {
		proxywasm.LogErrorf("read managed rate-limit executor response failed: %v", err)
		h.resumeAfterExecutorFailure(checks, "read executor response failed")
		return
	}
	var response executor.CheckResponse
	if err := json.Unmarshal(body, &response); err != nil {
		proxywasm.LogErrorf("parse managed rate-limit executor response failed: %v", err)
		h.resumeAfterExecutorFailure(checks, "parse executor response failed")
		return
	}
	if response.SchemaVersion != "" && response.SchemaVersion != executor.SchemaVersionV1 {
		h.resumeAfterExecutorFailure(checks, "unsupported executor response schema")
		return
	}
	if len(response.Results) != len(checks) {
		h.resumeAfterExecutorFailure(checks, "executor response result count mismatch")
		return
	}

	for i, result := range response.Results {
		check := checks[i]
		if result.Error != "" {
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
			return
		}
		decision := ratelimit.RedisResultDecision(
			check.Policy,
			check.Rule,
			check.Key,
			result.Allowed,
			result.Limit,
			result.Current,
			result.ResetSeconds,
		)
		if !decision.Allowed {
			sendRejected(decision)
			return
		}
		if len(decision.QuotaHeaders) > 0 {
			h.quotaHeaders = decision.QuotaHeaders
		}
	}
	_ = proxywasm.ResumeHttpRequest()
}

func (h *httpContext) handleExecutorFailure(checks []ratelimit.RedisCheck, message string) types.Action {
	proxywasm.LogErrorf("managed rate-limit executor failed: %s", message)
	for _, check := range checks {
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

func (h *httpContext) resumeAfterExecutorFailure(checks []ratelimit.RedisCheck, message string) {
	action := h.handleExecutorFailure(checks, message)
	if action == types.ActionContinue {
		_ = proxywasm.ResumeHttpRequest()
	}
}

func (h *httpContext) redisStore(name string) (config.RedisStore, bool) {
	for _, store := range h.plugin.config.RedisStores {
		if store.Name == name {
			return store, true
		}
	}
	return config.RedisStore{}, false
}

func executorTimeoutMillis(config *config.Executor, checks []ratelimit.RedisCheck) int {
	timeout := defaultExecutorTimeoutMillis
	if config.TimeoutMillis > timeout {
		timeout = config.TimeoutMillis
	}
	for _, check := range checks {
		if check.RedisTimeoutMs > timeout {
			timeout = check.RedisTimeoutMs
		}
	}
	return timeout
}

func httpCallStatus() int {
	headers, err := proxywasm.GetHttpCallResponseHeaders()
	if err != nil {
		return 0
	}
	for _, header := range headers {
		if header[0] != ":status" {
			continue
		}
		status, err := strconv.Atoi(header[1])
		if err != nil {
			return 0
		}
		return status
	}
	return 0
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
		aiModelHeaderName:  {},
		apiKeyHeaderName:   {},
		cookieHeaderName:   {},
		consumerHeaderName: {},
		tenantHeaderName:   {},
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
					if part.Type == config.KeyTypeJWTClaim && part.Name != "" {
						seen[jwtClaimHeaderPrefix+part.Name] = struct{}{}
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
