package compiler

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	httpwasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	pluginiprestriction "github.com/lgc202/ingate/pkg/plugin/iprestriction"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	httpRouterFilterName = "envoy.filters.http.router"
	wasmRuntime          = "envoy.wasm.runtime.v8"
)

// listenerFilterConfig 汇总一个 Listener 需要注入的内置治理插件
type listenerFilterConfig struct {
	ipRestriction *pluginiprestriction.PluginConfig
	rateLimit     *pluginratelimit.PluginConfig
}

func buildHTTPFilters(config listenerFilterConfig) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, 3)
	if config.ipRestriction != nil {
		filter, err := buildIPRestrictionHTTPFilter(config.ipRestriction)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	if config.rateLimit != nil {
		filter, err := buildRateLimitHTTPFilter(config.rateLimit)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	router, err := anypb.New(&routerv3.Router{})
	if err != nil {
		return nil, fmt.Errorf("encode Envoy router filter: %w", err)
	}
	filters = append(filters, &hcmv3.HttpFilter{
		Name:       httpRouterFilterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: router},
	})
	return filters, nil
}

func buildWasmHTTPFilter(
	filterName string,
	pluginName string,
	pluginPath string,
	raw []byte,
) (*hcmv3.HttpFilter, error) {
	configuration, err := anypb.New(&wrapperspb.StringValue{Value: string(raw)})
	if err != nil {
		return nil, fmt.Errorf("encode Wasm plugin configuration: %w", err)
	}
	pluginConfig := &wasmv3.PluginConfig{
		Name:          pluginName,
		RootId:        pluginName,
		Configuration: configuration,
		FailurePolicy: wasmv3.FailurePolicy_FAIL_CLOSED,
		Vm: &wasmv3.PluginConfig_VmConfig{
			VmConfig: &wasmv3.VmConfig{
				VmId:    pluginName,
				Runtime: wasmRuntime,
				Code: &corev3.AsyncDataSource{
					Specifier: &corev3.AsyncDataSource_Local{
						Local: &corev3.DataSource{
							Specifier: &corev3.DataSource_Filename{
								Filename: pluginPath,
							},
						},
					},
				},
			},
		},
	}
	wasmConfig := &httpwasmv3.Wasm{Config: pluginConfig}
	if err := wasmConfig.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate Wasm HTTP filter: %w", err)
	}
	typedConfig, err := anypb.New(wasmConfig)
	if err != nil {
		return nil, fmt.Errorf("encode Wasm HTTP filter: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name: filterName,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: typedConfig,
		},
	}, nil
}
