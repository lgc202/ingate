package server

import (
	"encoding/json"
	"slices"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

func mergeRateLimitConfig(current, next *pluginratelimit.Config) *pluginratelimit.Config {
	if next == nil {
		return current
	}
	if current == nil {
		return &pluginratelimit.Config{
			Bindings:    slices.Clone(next.Bindings),
			RedisStores: slices.Clone(next.RedisStores),
			DataPlane:   next.DataPlane,
		}
	}
	current.Bindings = append(current.Bindings, next.Bindings...)
	current.RedisStores = append(current.RedisStores, next.RedisStores...)
	if current.DataPlane == nil {
		current.DataPlane = next.DataPlane
	}
	return current
}

func (b responseBuilder) buildRateLimitHTTPFilter(runtimeConfig targetxds.Config) (*hcmv3.HttpFilter, error) {
	redisStores := uniqueRateLimitRedisStores(runtimeConfig.RateLimit.RedisStores)

	rawConfig, err := json.Marshal(pluginratelimit.PluginConfig{
		SchemaVersion: rateLimitSchemaVersion,
		RedisStores:   redisStores,
		DataPlane:     runtimeConfig.RateLimit.DataPlane,
		Routes:        rateLimitRouteConfigs(runtimeConfig.RouteConfigs, runtimeConfig.RateLimit),
	})
	if err != nil {
		return nil, err
	}

	typedConfig, err := b.buildWasmPluginTypedConfig(
		rateLimitPluginName,
		rateLimitPluginPath,
		string(rawConfig),
		wasmv3.FailurePolicy_FAIL_CLOSED,
	)
	if err != nil {
		return nil, err
	}

	return &hcmv3.HttpFilter{
		Name: rateLimitHTTPFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}

func uniqueRateLimitRedisStores(stores []pluginratelimit.RedisStore) []pluginratelimit.RedisStore {
	result := make([]pluginratelimit.RedisStore, 0, len(stores))
	seen := map[string]struct{}{}
	for _, store := range stores {
		if _, ok := seen[store.Name]; ok {
			continue
		}
		seen[store.Name] = struct{}{}
		result = append(result, store)
	}
	return result
}

func rateLimitRouteConfigs(configs []targetxds.RouteConfig, rateLimit *pluginratelimit.Config) []pluginratelimit.RouteConfig {
	result := make([]pluginratelimit.RouteConfig, 0)
	seen := map[string]struct{}{}
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				routeConfig, ok := buildRateLimitRouteConfig(route, rateLimit)
				if !ok {
					continue
				}
				key := routeRuntimeName(routeConfig.GatewayName, routeConfig.RouteName, routeConfig.RuleName, "")
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				result = append(result, routeConfig)
			}
		}
	}
	return result
}

func buildRateLimitRouteConfig(route targetxds.Route, rateLimit *pluginratelimit.Config) (pluginratelimit.RouteConfig, bool) {
	if rateLimit == nil {
		return pluginratelimit.RouteConfig{}, false
	}

	bindings := make([]pluginratelimit.Binding, 0)
	for _, binding := range rateLimit.Bindings {
		if !rateLimitBindingMatchesRoute(binding, route) {
			continue
		}
		bindings = append(bindings, binding)
	}
	if len(bindings) == 0 {
		return pluginratelimit.RouteConfig{}, false
	}

	return pluginratelimit.RouteConfig{
		SchemaVersion: rateLimitSchemaVersion,
		GatewayName:   route.GatewayName,
		RouteName:     route.Name,
		RuleName:      route.RuleName,
		Bindings:      bindings,
	}, true
}

func rateLimitBindingMatchesRoute(binding pluginratelimit.Binding, route targetxds.Route) bool {
	switch resource.Kind(binding.Target.Kind) {
	case resource.KindGateway:
		return binding.Target.Name == route.GatewayName
	case resource.KindRoute:
		return binding.Target.Name == route.Name && (binding.Target.RuleName == "" || binding.Target.RuleName == route.RuleName)
	default:
		return false
	}
}
