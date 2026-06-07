package server

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	httpConnectionManagerFilterName = "envoy.filters.network.http_connection_manager"
	httpRouterFilterName            = "envoy.filters.http.router"
	httpWasmFilterName              = "envoy.filters.http.wasm"
	managedRateLimitHTTPFilterName  = "ingate.filters.http.managed_rate_limit"
	managedRateLimitPluginName      = "ingate.managed.rate-limit"
	managedRateLimitPluginPath      = "/opt/ingate/plugins/rate-limit.wasm"
	managedRateLimitSchemaVersion   = "v1"
	wasmRuntime                     = "envoy.wasm.runtime.v8"
	defaultBindAddress              = "0.0.0.0"
)

type managedRateLimitFilterConfig struct {
	SchemaVersion string                          `json:"schemaVersion"`
	RedisStores   []targetxds.RateLimitRedisStore `json:"redisStores,omitempty"`
	Routes        []managedRateLimitRouteConfig   `json:"routes,omitempty"`
}

func (b responseBuilder) buildListeners(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	groups := b.listenerGroups(configs)
	for _, key := range sortedListenerKeys(groups.keys) {
		config := groups.config(key)
		httpFilters, err := b.buildHTTPFilters(config)
		if err != nil {
			return nil, err
		}

		listenerName := listenerGroupName(key)
		hcm, err := anypb.New(&hcmv3.HttpConnectionManager{
			CodecType:  hcmv3.HttpConnectionManager_AUTO,
			StatPrefix: listenerName,
			RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
				Rds: &hcmv3.Rds{
					ConfigSource:    b.adsConfigSource(),
					RouteConfigName: listenerRouteConfigName(key),
				},
			},
			HttpFilters: httpFilters,
		})
		if err != nil {
			return nil, err
		}

		resource, err := anypb.New(&listenerv3.Listener{
			Name:    listenerName,
			Address: b.socketAddress(defaultBindAddress, key.port),
			FilterChains: []*listenerv3.FilterChain{
				{
					Filters: []*listenerv3.Filter{
						{
							Name: httpConnectionManagerFilterName,
							ConfigType: &listenerv3.Filter_TypedConfig{
								TypedConfig: hcm,
							},
						},
					},
				},
			},
		})
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

type listenerGroups struct {
	keys    map[listenerGroupKey]struct{}
	configs map[listenerGroupKey]targetxds.Config
}

func (b responseBuilder) listenerGroups(configs []snapshotConfig) listenerGroups {
	groups := listenerGroups{
		keys:    map[listenerGroupKey]struct{}{},
		configs: map[listenerGroupKey]targetxds.Config{},
	}

	for _, snapshot := range configs {
		seen := map[listenerGroupKey]struct{}{}
		for _, listener := range snapshot.Config.Listeners {
			key := listenerKey(listener)
			groups.keys[key] = struct{}{}

			config := groups.configs[key]
			config.RouteConfigs = append(config.RouteConfigs, routeConfigsForListener(snapshot.Config.RouteConfigs, listener.RouteConfigName)...)
			if _, ok := seen[key]; ok {
				groups.configs[key] = config
				continue
			}
			seen[key] = struct{}{}

			config.Plugins = append(config.Plugins, snapshot.Config.Plugins...)
			config.PluginBindings = append(config.PluginBindings, snapshot.Config.PluginBindings...)
			config.ManagedRateLimit = mergeManagedRateLimit(config.ManagedRateLimit, snapshot.Config.ManagedRateLimit)
			groups.configs[key] = config
		}
	}
	return groups
}

func (g listenerGroups) config(key listenerGroupKey) targetxds.Config {
	return g.configs[key]
}

func routeConfigsForListener(configs []targetxds.RouteConfig, name string) []targetxds.RouteConfig {
	result := make([]targetxds.RouteConfig, 0, 1)
	for _, config := range configs {
		if config.Name == name {
			result = append(result, config)
		}
	}
	return result
}

func (b responseBuilder) buildHTTPFilters(config targetxds.Config) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, len(config.PluginBindings)+2)
	if config.ManagedRateLimit != nil {
		filter, err := b.buildManagedRateLimitHTTPFilter(config)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	pluginByName := make(map[string]targetxds.Plugin, len(config.Plugins))
	for _, plugin := range config.Plugins {
		pluginByName[plugin.Name] = plugin
	}

	bindings := slices.Clone(config.PluginBindings)
	slices.SortFunc(bindings, func(a, b targetxds.PluginBinding) int {
		return cmp.Compare(a.Priority, b.Priority)
	})
	for _, binding := range bindings {
		for _, pluginRef := range binding.Plugins {
			plugin, ok := pluginByName[pluginRef.Name]
			if !ok || plugin.Runtime != resource.PluginRuntimeWasm {
				continue
			}
			filter, err := b.buildWasmHTTPFilter(binding, plugin, pluginRef)
			if err != nil {
				return nil, err
			}
			filters = append(filters, filter)
		}
	}

	filters = append(filters, &hcmv3.HttpFilter{
		Name: httpRouterFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: b.mustAny(&routerv3.Router{}),
		},
	})
	return filters, nil
}

func mergeManagedRateLimit(current, next *targetxds.ManagedRateLimit) *targetxds.ManagedRateLimit {
	if next == nil {
		return current
	}
	if current == nil {
		return &targetxds.ManagedRateLimit{
			Bindings:    slices.Clone(next.Bindings),
			RedisStores: slices.Clone(next.RedisStores),
		}
	}
	current.Bindings = append(current.Bindings, next.Bindings...)
	current.RedisStores = append(current.RedisStores, next.RedisStores...)
	return current
}

func managedRateLimitRouteConfigs(configs []targetxds.RouteConfig, rateLimit *targetxds.ManagedRateLimit) []managedRateLimitRouteConfig {
	result := make([]managedRateLimitRouteConfig, 0)
	seen := map[string]struct{}{}
	for _, config := range configs {
		for _, virtualHost := range config.VirtualHosts {
			for _, route := range virtualHost.Routes {
				routeConfig, ok := buildManagedRateLimitRouteConfig(route, rateLimit)
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

func (b responseBuilder) buildManagedRateLimitHTTPFilter(runtimeConfig targetxds.Config) (*hcmv3.HttpFilter, error) {
	redisStores := make([]targetxds.RateLimitRedisStore, 0, len(runtimeConfig.ManagedRateLimit.RedisStores))
	seenRedisStores := map[string]struct{}{}
	for _, store := range runtimeConfig.ManagedRateLimit.RedisStores {
		if _, ok := seenRedisStores[store.Name]; ok {
			continue
		}
		seenRedisStores[store.Name] = struct{}{}
		redisStores = append(redisStores, store)
	}

	rawConfig, err := json.Marshal(managedRateLimitFilterConfig{
		SchemaVersion: managedRateLimitSchemaVersion,
		RedisStores:   redisStores,
		Routes:        managedRateLimitRouteConfigs(runtimeConfig.RouteConfigs, runtimeConfig.ManagedRateLimit),
	})
	if err != nil {
		return nil, err
	}

	typedConfig, err := b.buildWasmPluginTypedConfig(
		managedRateLimitPluginName,
		managedRateLimitPluginPath,
		string(rawConfig),
		wasmv3.FailurePolicy_FAIL_CLOSED,
	)
	if err != nil {
		return nil, err
	}

	return &hcmv3.HttpFilter{
		Name: managedRateLimitHTTPFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}

func (b responseBuilder) buildWasmHTTPFilter(binding targetxds.PluginBinding, plugin targetxds.Plugin, pluginRef targetxds.PluginRef) (*hcmv3.HttpFilter, error) {
	if plugin.Image == "" {
		return nil, fmt.Errorf("wasm plugin %q has no image", plugin.Name)
	}

	typedConfig, err := b.buildWasmPluginTypedConfig(
		plugin.Name,
		plugin.Image,
		string(pluginRef.Config),
		b.wasmFailurePolicy(binding.FailurePolicy),
	)
	if err != nil {
		return nil, err
	}

	return &hcmv3.HttpFilter{
		Name: httpWasmFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}

func (b responseBuilder) buildWasmPluginTypedConfig(name, image, configuration string, failurePolicy wasmv3.FailurePolicy) (*anypb.Any, error) {
	config, err := anypb.New(&wrapperspb.StringValue{Value: configuration})
	if err != nil {
		return nil, err
	}

	typedConfig, err := anypb.New(&httpwasmv3.Wasm{
		Config: &wasmv3.PluginConfig{
			Name:          name,
			RootId:        name,
			Configuration: config,
			FailurePolicy: failurePolicy,
			Vm: &wasmv3.PluginConfig_VmConfig{
				VmConfig: &wasmv3.VmConfig{
					VmId:    name,
					Runtime: wasmRuntime,
					Code: &corev3.AsyncDataSource{
						Specifier: &corev3.AsyncDataSource_Local{
							Local: &corev3.DataSource{
								Specifier: &corev3.DataSource_Filename{
									Filename: image,
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return typedConfig, nil
}

func (b responseBuilder) wasmFailurePolicy(policy resource.PluginFailurePolicy) wasmv3.FailurePolicy {
	switch policy {
	case resource.PluginFailurePolicyFailOpen, resource.PluginFailurePolicySkipAndLog:
		return wasmv3.FailurePolicy_FAIL_OPEN
	case resource.PluginFailurePolicyFailClose:
		return wasmv3.FailurePolicy_FAIL_CLOSED
	default:
		return wasmv3.FailurePolicy_UNSPECIFIED
	}
}
