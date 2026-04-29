package server

import (
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	httpConnectionManagerFilterName = "envoy.filters.network.http_connection_manager"
	httpRouterFilterName            = "envoy.filters.http.router"
	defaultBindAddress              = "0.0.0.0"
)

func (b responseBuilder) buildListeners(configs []snapshotConfig) ([]*anypb.Any, error) {
	resources := make([]*anypb.Any, 0)
	for _, config := range configs {
		for _, listener := range config.Config.Listeners {
			hcm, err := anypb.New(&hcmv3.HttpConnectionManager{
				CodecType:  hcmv3.HttpConnectionManager_AUTO,
				StatPrefix: listener.Name,
				RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
					Rds: &hcmv3.Rds{
						ConfigSource:    b.adsConfigSource(),
						RouteConfigName: listener.RouteConfigName,
					},
				},
				HttpFilters: []*hcmv3.HttpFilter{
					{
						Name: httpRouterFilterName,
						ConfigType: &hcmv3.HttpFilter_TypedConfig{
							TypedConfig: b.mustAny(&routerv3.Router{}),
						},
					},
				},
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
