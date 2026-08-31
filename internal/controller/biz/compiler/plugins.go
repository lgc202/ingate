package compiler

import (
	"cmp"
	"fmt"
	"slices"

	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

const httpRouterFilterName = "envoy.filters.http.router"

// listenerFilterConfig 记录只在部分 Listener 上启用的过滤器
type listenerFilterConfig struct {
	ipRestriction bool
	wasm          []wasmFilter
}

func buildHTTPFilters(config listenerFilterConfig) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, 4+len(config.wasm))
	// 顺序决定同一请求的执行语义：先拒绝非法来源并完成调用方识别，
	// 再执行治理策略，最后进入 AI 协议处理。
	if config.ipRestriction {
		filter, err := buildIPRestrictionHTTPFilter()
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	authorization, err := buildAuthorizationHTTPFilter()
	if err != nil {
		return nil, err
	}
	filters = append(filters, authorization)
	wasmFilters := slices.Clone(config.wasm)
	slices.SortStableFunc(wasmFilters, func(left, right wasmFilter) int {
		if order := cmp.Compare(left.phase, right.phase); order != 0 {
			return order
		}
		return cmp.Compare(left.name, right.name)
	})
	for _, wasm := range wasmFilters {
		filter, err := buildWasmHTTPFilter(wasm)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	aiExtProc, err := buildAIDownstreamExtProcHTTPFilter()
	if err != nil {
		return nil, err
	}
	filters = append(filters, aiExtProc)

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
