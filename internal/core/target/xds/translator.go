// Package xds 将逻辑网关模型翻译成 Envoy-oriented 配置
package xds

import (
	"fmt"
	"slices"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/runtime"
)

// Translator 负责生成 xDS target 的配置快照
type Translator struct{}

// Config 表示 xDS target 的内部配置载荷
type Config struct {
	Listeners    []Listener
	RouteConfigs []RouteConfig
	Clusters     []Cluster
}

// Listener 表示 Envoy listener 的内部模型
type Listener struct {
	Name            string
	Protocol        string
	Port            int
	Hostname        string
	RouteConfigName string
}

// RouteConfig 表示 Envoy route configuration 的内部模型
type RouteConfig struct {
	Name         string
	VirtualHosts []VirtualHost
}

// VirtualHost 表示 Envoy virtual host 的内部模型
type VirtualHost struct {
	Name    string
	Domains []string
	Routes  []Route
}

// Route 表示 Envoy route 的内部模型
type Route struct {
	Name             string
	Match            RouteMatch
	WeightedClusters []WeightedCluster
}

// RouteMatch 表示 Envoy route match 的内部模型
type RouteMatch struct {
	PathPrefix string
}

// WeightedCluster 表示 Envoy weighted cluster 的内部模型
type WeightedCluster struct {
	Name   string
	Weight int
}

// Cluster 表示 Envoy cluster 的内部模型
type Cluster struct {
	Name      string
	Endpoints []Endpoint
}

// Endpoint 表示 Envoy endpoint 的内部模型
type Endpoint struct {
	Address string
	Port    int
}

// Target 返回运行时 target 名称
func (Translator) Target() string {
	return "xds"
}

// Translate 将逻辑网关模型转换成 xDS 运行时快照
func (t Translator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	config := Config{
		Listeners:    make([]Listener, 0, len(logical.Listeners)),
		RouteConfigs: make([]RouteConfig, 0, len(logical.Listeners)),
		Clusters:     make([]Cluster, 0, len(logical.Upstreams)),
	}

	for _, listener := range logical.Listeners {
		routeConfigName := fmt.Sprintf("%s/%s/routes", logical.Name, listener.Name)
		config.Listeners = append(config.Listeners, Listener{
			Name:            fmt.Sprintf("%s/%s", logical.Name, listener.Name),
			Protocol:        listener.Protocol,
			Port:            listener.Port,
			Hostname:        listener.Hostname,
			RouteConfigName: routeConfigName,
		})

		routeConfig := RouteConfig{
			Name:         routeConfigName,
			VirtualHosts: make([]VirtualHost, 0, len(logical.Routes)),
		}
		for _, route := range logical.Routes {
			virtualHost := VirtualHost{
				Name:    route.Name,
				Domains: slices.Clone(route.Hostnames),
				Routes:  make([]Route, 0, len(route.Rules)),
			}
			for _, rule := range route.Rules {
				xdsRoute := Route{
					Name: route.Name,
					Match: RouteMatch{
						PathPrefix: rule.PathPrefix,
					},
					WeightedClusters: make([]WeightedCluster, 0, len(rule.Upstreams)),
				}
				for _, upstream := range rule.Upstreams {
					xdsRoute.WeightedClusters = append(xdsRoute.WeightedClusters, WeightedCluster{
						Name:   upstream.Name,
						Weight: upstream.Weight,
					})
				}
				virtualHost.Routes = append(virtualHost.Routes, xdsRoute)
			}
			routeConfig.VirtualHosts = append(routeConfig.VirtualHosts, virtualHost)
		}
		config.RouteConfigs = append(config.RouteConfigs, routeConfig)
	}

	for _, upstream := range logical.Upstreams {
		cluster := Cluster{
			Name:      upstream.Name,
			Endpoints: make([]Endpoint, 0, len(upstream.Endpoints)),
		}
		for _, endpoint := range upstream.Endpoints {
			cluster.Endpoints = append(cluster.Endpoints, Endpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
			})
		}
		config.Clusters = append(config.Clusters, cluster)
	}

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf("xds/%s", logical.Name),
		Config:  config,
	}, nil
}
