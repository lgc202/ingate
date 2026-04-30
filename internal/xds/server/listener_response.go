package server

import (
	"cmp"
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
	wasmRuntime                     = "envoy.wasm.runtime.v8"
	defaultBindAddress              = "0.0.0.0"
)

func (b responseBuilder) buildListeners(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	for _, config := range configs {
		for _, listener := range config.Config.Listeners {
			httpFilters, err := b.buildHTTPFilters(config.Config)
			if err != nil {
				return nil, err
			}

			hcm, err := anypb.New(&hcmv3.HttpConnectionManager{
				CodecType:  hcmv3.HttpConnectionManager_AUTO,
				StatPrefix: listener.Name,
				RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
					Rds: &hcmv3.Rds{
						ConfigSource:    b.adsConfigSource(),
						RouteConfigName: listener.RouteConfigName,
					},
				},
				HttpFilters: httpFilters,
			})
			if err != nil {
				return nil, err
			}

			resource, err := anypb.New(&listenerv3.Listener{
				Name:    listener.Name,
				Address: b.socketAddress(defaultBindAddress, listener.Port),
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
	}
	return resources, nil
}

func (b responseBuilder) buildHTTPFilters(config targetxds.Config) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, len(config.PluginBindings)+1)
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

func (b responseBuilder) buildWasmHTTPFilter(binding targetxds.PluginBinding, plugin targetxds.Plugin, pluginRef targetxds.PluginRef) (*hcmv3.HttpFilter, error) {
	if plugin.Image == "" {
		return nil, fmt.Errorf("wasm plugin %q has no image", plugin.Name)
	}

	config, err := anypb.New(&wrapperspb.StringValue{Value: string(pluginRef.Config)})
	if err != nil {
		return nil, err
	}

	typedConfig, err := anypb.New(&httpwasmv3.Wasm{
		Config: &wasmv3.PluginConfig{
			Name:          plugin.Name,
			RootId:        plugin.Name,
			Configuration: config,
			FailurePolicy: b.wasmFailurePolicy(binding.FailurePolicy),
			Vm: &wasmv3.PluginConfig_VmConfig{
				VmConfig: &wasmv3.VmConfig{
					VmId:    plugin.Name,
					Runtime: wasmRuntime,
					Code: &corev3.AsyncDataSource{
						Specifier: &corev3.AsyncDataSource_Local{
							Local: &corev3.DataSource{
								Specifier: &corev3.DataSource_Filename{
									Filename: plugin.Image,
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

	return &hcmv3.HttpFilter{
		Name: httpWasmFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
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
