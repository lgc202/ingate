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

func mergeRateLimitConfig(current, next *targetxds.RateLimitConfig) *targetxds.RateLimitConfig {
	if next == nil {
		return current
	}
	if current == nil {
		return &targetxds.RateLimitConfig{
			Bindings: slices.Clone(next.Bindings),
		}
	}
	current.Bindings = append(current.Bindings, next.Bindings...)
	return current
}

func (b responseBuilder) buildRateLimitHTTPFilter(runtimeConfig targetxds.Config) (*hcmv3.HttpFilter, error) {
	rawConfig, err := json.Marshal(pluginratelimit.PluginConfig{
		Routes: rateLimitRouteConfigs(runtimeConfig.RouteConfigs, runtimeConfig.RateLimit),
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

func rateLimitRouteConfigs(configs []targetxds.RouteConfig, rateLimit *targetxds.RateLimitConfig) []pluginratelimit.RouteConfig {
	result := make([]pluginratelimit.RouteConfig, 0)
	seen := map[string]struct{}{}
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				routeConfig, ok := buildRateLimitRouteConfig(route, rateLimit)
				if !ok {
					continue
				}
				key := routeConfig.GatewayName + "\x00" + routeConfig.RouteName
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

func buildRateLimitRouteConfig(route targetxds.Route, rateLimit *targetxds.RateLimitConfig) (pluginratelimit.RouteConfig, bool) {
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
		GatewayName: route.GatewayName,
		RouteName:   route.Name,
		Bindings:    bindings,
	}, true
}

func rateLimitBindingMatchesRoute(binding pluginratelimit.Binding, route targetxds.Route) bool {
	switch resource.Kind(binding.Target.Kind) {
	case resource.KindGateway:
		return binding.Target.Name == route.GatewayName
	case resource.KindRoute:
		return binding.Target.Name == route.Name
	default:
		return false
	}
}
