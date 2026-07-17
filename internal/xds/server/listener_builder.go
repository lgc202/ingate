package server

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	targetxds "github.com/lgc202/ingate/internal/core/target/xds"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	httpConnectionManagerFilterName = "envoy.filters.network.http_connection_manager"
	httpRouterFilterName            = "envoy.filters.http.router"
	httpWasmFilterName              = "envoy.filters.http.wasm"
	accessControlHTTPFilterName     = "ingate.filters.http.acl"
	accessControlPluginName         = "ingate.acl"
	accessControlPluginPath         = "/opt/ingate/plugins/acl.wasm"
	accessControlSchemaVersion      = "v1"
	rateLimitHTTPFilterName         = "ingate.filters.http.ratelimit"
	rateLimitPluginName             = "ingate.ratelimit"
	rateLimitPluginPath             = "/opt/ingate/plugins/ratelimit.wasm"
	wasmRuntime                     = "envoy.wasm.runtime.v8"
	defaultBindAddress              = "0.0.0.0"
)

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

			config.RateLimit = mergeRateLimitConfig(config.RateLimit, snapshot.Config.RateLimit)
			config.AccessControl = mergeAccessControlConfig(config.AccessControl, snapshot.Config.AccessControl)
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
	filters := make([]*hcmv3.HttpFilter, 0, 3)
	if config.AccessControl != nil {
		filter, err := b.buildAccessControlHTTPFilter(config)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	if config.RateLimit != nil {
		filter, err := b.buildRateLimitHTTPFilter(config)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}

	filters = append(filters, &hcmv3.HttpFilter{
		Name: httpRouterFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: b.mustAny(&routerv3.Router{}),
		},
	})
	return filters, nil
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
