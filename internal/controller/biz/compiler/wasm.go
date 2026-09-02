package compiler

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	wasmfilterv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/wasm/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	wasmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/wasm/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	wasmHTTPFilterNamePrefix = "ingate.wasm/"
	wasmVMRuntime            = "envoy.wasm.runtime.v8"
)

type wasmFilterPhase uint8

const (
	// Header 等流量变换必须先运行，后续插件才能看到变换后的请求
	wasmFilterPhaseTrafficMutation wasmFilterPhase = iota + 1
	// 本地响应会终止请求，必须位于鉴权和流量变换之后
	wasmFilterPhaseLocalResponse
)

// wasmFilter 是强类型策略编译后的内部执行配置，不进入用户 API
type wasmFilter struct {
	name          string
	phase         wasmFilterPhase
	vmID          string
	rootID        string
	configuration []byte
	module        WasmModule
}

func buildWasmHTTPFilter(filter wasmFilter) (*hcmv3.HttpFilter, error) {
	configuration, err := anypb.New(wrapperspb.String(string(filter.configuration)))
	if err != nil {
		return nil, fmt.Errorf("encode Wasm filter %q configuration: %w", filter.name, err)
	}
	pluginConfig := &wasmv3.PluginConfig{
		Name:          filter.name,
		RootId:        filter.rootID,
		Configuration: configuration,
		// 强类型策略属于网关执行语义，插件异常时不能静默绕过规则
		FailurePolicy: wasmv3.FailurePolicy_FAIL_CLOSED,
		Vm: &wasmv3.PluginConfig_VmConfig{VmConfig: &wasmv3.VmConfig{
			VmId:    filter.vmID,
			Runtime: wasmVMRuntime,
			// Controller 在发布 xDS 前已经完成下载、ABI 校验和原子写入，
			// Envoy 只读取共享目录中的不可变文件。
			Code: &corev3.AsyncDataSource{Specifier: &corev3.AsyncDataSource_Local{
				Local: &corev3.DataSource{Specifier: &corev3.DataSource_Filename{
					Filename: filter.module.Path,
				}},
			}},
		}},
	}
	typedConfig, err := anypb.New(&wasmfilterv3.Wasm{Config: pluginConfig})
	if err != nil {
		return nil, fmt.Errorf("encode Wasm filter %q: %w", filter.name, err)
	}
	return &hcmv3.HttpFilter{
		Name:       wasmFilterName(filter.name),
		Disabled:   true,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: typedConfig},
	}, nil
}

func enableWasmOnRoute(route *routev3.Route, filterName string) error {
	if route.TypedPerFilterConfig == nil {
		route.TypedPerFilterConfig = make(map[string]*anypb.Any)
	}
	if _, exists := route.TypedPerFilterConfig[filterName]; exists {
		return fmt.Errorf("filter %q is already configured", filterName)
	}
	config, err := anypb.New(&routev3.FilterConfig{Config: &anypb.Any{}})
	if err != nil {
		return err
	}
	route.TypedPerFilterConfig[filterName] = config
	return nil
}

func wasmFilterName(name string) string {
	return wasmHTTPFilterNamePrefix + name
}
