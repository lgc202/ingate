package server

import (
	"encoding/json"

	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginacl "github.com/lgc202/ingate/pkg/plugin/acl"
)

func mergeAccessControlConfig(current, next *targetxds.AccessControlConfig) *targetxds.AccessControlConfig {
	if next == nil {
		return current
	}
	if current == nil {
		return &targetxds.AccessControlConfig{
			Bindings: append([]pluginacl.Binding(nil), next.Bindings...),
		}
	}
	current.Bindings = append(current.Bindings, next.Bindings...)
	return current
}

func (b responseBuilder) buildAccessControlHTTPFilter(runtimeConfig targetxds.Config) (*hcmv3.HttpFilter, error) {
	rawConfig, err := json.Marshal(pluginacl.PluginConfig{
		SchemaVersion: accessControlSchemaVersion,
		Routes:        accessControlRouteConfigs(runtimeConfig.RouteConfigs, runtimeConfig.AccessControl),
	})
	if err != nil {
		return nil, err
	}

	typedConfig, err := b.buildWasmPluginTypedConfig(
		accessControlPluginName,
		accessControlPluginPath,
		string(rawConfig),
		wasmv3.FailurePolicy_FAIL_CLOSED,
	)
	if err != nil {
		return nil, err
	}

	return &hcmv3.HttpFilter{
		Name: accessControlHTTPFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}

func accessControlRouteConfigs(configs []targetxds.RouteConfig, accessControl *targetxds.AccessControlConfig) []pluginacl.RouteConfig {
	result := make([]pluginacl.RouteConfig, 0)
	seen := map[string]struct{}{}
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				routeConfig, ok := buildAccessControlRouteConfig(route, accessControl)
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

func buildAccessControlRouteConfig(route targetxds.Route, accessControl *targetxds.AccessControlConfig) (pluginacl.RouteConfig, bool) {
	if accessControl == nil {
		return pluginacl.RouteConfig{}, false
	}

	bindings := make([]pluginacl.Binding, 0)
	for _, binding := range accessControl.Bindings {
		if !accessControlBindingMatchesRoute(binding, route) {
			continue
		}
		bindings = append(bindings, binding)
	}
	if len(bindings) == 0 {
		return pluginacl.RouteConfig{}, false
	}

	return pluginacl.RouteConfig{
		SchemaVersion: accessControlSchemaVersion,
		GatewayName:   route.GatewayName,
		RouteName:     route.Name,
		RuleName:      route.RuleName,
		Bindings:      bindings,
	}, true
}

func accessControlBindingMatchesRoute(binding pluginacl.Binding, route targetxds.Route) bool {
	switch resource.Kind(binding.Target.Kind) {
	case resource.KindGateway:
		return binding.Target.Name == route.GatewayName
	case resource.KindRoute:
		return binding.Target.Name == route.Name && (binding.Target.RuleName == "" || binding.Target.RuleName == route.RuleName)
	default:
		return false
	}
}
