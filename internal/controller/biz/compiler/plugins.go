package compiler

import (
	"fmt"

	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

const httpRouterFilterName = "envoy.filters.http.router"

// listenerFilterConfig 记录只在部分 Listener 上启用的过滤器
type listenerFilterConfig struct {
	ipRestriction bool
}

func buildHTTPFilters(config listenerFilterConfig) ([]*hcmv3.HttpFilter, error) {
	filters := make([]*hcmv3.HttpFilter, 0, 4)
	if config.ipRestriction {
		filter, err := buildIPRestrictionHTTPFilter()
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	callerAuth, err := buildCallerAuthHTTPFilter()
	if err != nil {
		return nil, err
	}
	filters = append(filters, callerAuth)
	aiExtProc, err := buildAIEntryExtProcHTTPFilter()
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
