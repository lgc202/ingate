package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/config"
	"github.com/lgc202/ingate/plugins/managed/rate-limit/internal/ratelimit"
	"github.com/tidwall/resp"
)

func main() {}

func init() {
	wrapper.SetCtx(
		"ingate-managed-rate-limit",
		wrapper.ParseOverrideRawConfig(parseGlobalConfig, parseRouteConfig),
		wrapper.ProcessRequestHeaders(onHTTPRequestHeaders),
		wrapper.ProcessResponseHeaders(onHTTPResponseHeaders),
	)
}

const (
	defaultRedisPort          = 6379
	defaultRedisTimeoutMillis = 1000
	quotaHeadersContextKey    = "ingate.rate-limit.quota-headers"
	cookieHeaderName          = "cookie"
	consumerHeaderName        = "x-ingate-consumer"

	fixedWindowScript = `
local key = KEYS[1]
local threshold = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = tonumber(redis.call('get', key) or "0")
if current > threshold then
	return {threshold, current, redis.call('ttl', key)}
end

current = redis.call('incr', key)
if current == 1 then
	redis.call('expire', key, window)
end

return {threshold, current, redis.call('ttl', key)}
`
)

type runtimeConfig struct {
	plugin      config.PluginConfig
	route       config.RouteConfig
	redisClient map[string]wrapper.RedisClient
}

var localLimiter = ratelimit.NewLocalLimiter()

func parseGlobalConfig(configBytes []byte, cfg *runtimeConfig) error {
	pluginConfig, err := config.ParsePluginConfig(configBytes)
	if err != nil {
		return err
	}
	redisClients, err := initRedisClients(pluginConfig.RedisStores)
	if err != nil {
		return err
	}
	cfg.plugin = pluginConfig
	cfg.redisClient = redisClients
	return nil
}

func parseRouteConfig(configBytes []byte, global runtimeConfig, cfg *runtimeConfig) error {
	routeConfig, err := config.ParseRouteConfig(configBytes)
	if err != nil {
		return err
	}
	cfg.plugin = global.plugin
	cfg.redisClient = global.redisClient
	cfg.route = routeConfig
	return nil
}

func initRedisClients(stores []config.RedisStore) (map[string]wrapper.RedisClient, error) {
	clients := make(map[string]wrapper.RedisClient, len(stores))
	for _, store := range stores {
		host, port, err := splitRedisAddress(store.Address)
		if err != nil {
			return nil, err
		}
		timeout := store.ConnectTimeoutMillis
		if timeout <= 0 {
			timeout = defaultRedisTimeoutMillis
		}
		if store.TLS {
			log.Warnf("redis store %s enables TLS, but current managed rate-limit plugin does not enable TLS transport yet", store.Name)
		}
		if store.PasswordRef != "" {
			log.Warnf("redis store %s uses passwordRef %s, secret resolution is not wired into the plugin yet", store.Name, store.PasswordRef)
		}
		client := wrapper.NewRedisClusterClient(wrapper.FQDNCluster{
			FQDN: host,
			Port: int64(port),
		})
		if err := client.Init(store.Username, "", int64(timeout), wrapper.WithDataBase(store.DB)); err != nil {
			return nil, err
		}
		clients[store.Name] = client
	}
	return clients, nil
}

func onHTTPRequestHeaders(ctx wrapper.HttpContext, cfg runtimeConfig) types.Action {
	if len(cfg.route.Bindings) == 0 {
		return types.ActionContinue
	}

	req := requestFromProxyWasm(cfg.route)
	result := localLimiter.Evaluate(cfg.route, req)
	if !result.Allowed {
		sendRejected(result.Decision)
		return types.ActionPause
	}
	if len(result.QuotaHeaders) > 0 {
		ctx.SetContext(quotaHeadersContextKey, result.QuotaHeaders)
	}
	if len(result.RedisChecks) == 0 {
		return types.ActionContinue
	}

	runRedisChecks(ctx, cfg, result.RedisChecks, 0)
	return types.HeaderStopAllIterationAndWatermark
}

func onHTTPResponseHeaders(ctx wrapper.HttpContext, _ runtimeConfig) types.Action {
	headers, ok := ctx.GetContext(quotaHeadersContextKey).(map[string]string)
	if !ok {
		return types.ActionContinue
	}
	for name, value := range headers {
		_ = proxywasm.ReplaceHttpResponseHeader(name, value)
	}
	return types.ActionContinue
}

func runRedisChecks(ctx wrapper.HttpContext, cfg runtimeConfig, checks []ratelimit.RedisCheck, index int) {
	if index >= len(checks) {
		proxywasm.ResumeHttpRequest()
		return
	}

	check := checks[index]
	client := cfg.redisClient[check.RedisStore]
	if client == nil {
		handleRedisError(ctx, cfg, checks, index, fmt.Errorf("redis store %q is not configured", check.RedisStore))
		return
	}

	keys := []interface{}{check.RedisKey}
	args := []interface{}{check.Requests, check.WindowSeconds}
	err := client.Eval(fixedWindowScript, 1, keys, args, func(response resp.Value) {
		values := response.Array()
		if len(values) != 3 {
			handleRedisError(ctx, cfg, checks, index, fmt.Errorf("unexpected redis response: %v", response))
			return
		}
		threshold := int(values[0].Integer())
		current := int(values[1].Integer())
		reset := int(values[2].Integer())
		decision := ratelimit.RedisDecision(check.Policy, check.Rule, check.Key, threshold, current, reset)
		if !decision.Allowed {
			sendRejected(decision)
			return
		}
		if len(decision.QuotaHeaders) > 0 {
			ctx.SetContext(quotaHeadersContextKey, decision.QuotaHeaders)
		}
		runRedisChecks(ctx, cfg, checks, index+1)
	})
	if err != nil {
		handleRedisError(ctx, cfg, checks, index, err)
	}
}

func handleRedisError(ctx wrapper.HttpContext, cfg runtimeConfig, checks []ratelimit.RedisCheck, index int, err error) {
	check := checks[index]
	log.Errorf("managed rate-limit redis check failed: policy=%s rule=%s redisStore=%s err=%v", check.Policy.Name, check.Rule.Name, check.RedisStore, err)
	if check.Policy.FailOpen() {
		runRedisChecks(ctx, cfg, checks, index+1)
		return
	}
	sendRejected(ratelimit.Decision{
		Allowed:    false,
		StatusCode: check.Policy.RejectedStatusCode(),
		Message:    check.Policy.RejectedMessage(),
		Policy:     check.Policy,
		Rule:       check.Rule,
		Key:        check.Key,
	})
}

func sendRejected(decision ratelimit.Decision) {
	_ = proxywasm.SendHttpResponseWithDetail(
		uint32(decision.StatusCode),
		"ingate.rate_limit.rejected",
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

func splitRedisAddress(address string) (string, int, error) {
	host, portValue, err := net.SplitHostPort(address)
	if err == nil {
		port, err := strconv.Atoi(portValue)
		if err != nil {
			return "", 0, err
		}
		return host, port, nil
	}
	if address == "" {
		return "", 0, fmt.Errorf("redis address is empty")
	}
	return strings.Trim(address, "[]"), defaultRedisPort, nil
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
