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
	Listeners           []Listener           `json:"listeners"`
	RouteConfigs        []RouteConfig        `json:"routeConfigs"`
	Clusters            []Cluster            `json:"clusters"`
	EndpointAssignments []EndpointAssignment `json:"endpointAssignments"`
}

// Listener 表示 Envoy listener 的内部模型
type Listener struct {
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	Port            int    `json:"port"`
	Hostname        string `json:"hostname"`
	RouteConfigName string `json:"routeConfigName"`
}

// RouteConfig 表示 Envoy route configuration 的内部模型
type RouteConfig struct {
	Name         string        `json:"name"`
	VirtualHosts []VirtualHost `json:"virtualHosts"`
}

// VirtualHost 表示 Envoy virtual host 的内部模型
type VirtualHost struct {
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
	Routes  []Route  `json:"routes"`
}

// Route 表示 Envoy route 的内部模型
type Route struct {
	Name             string            `json:"name"`
	Match            RouteMatch        `json:"match"`
	TimeoutMillis    int               `json:"timeoutMillis"`
	WeightedClusters []WeightedCluster `json:"weightedClusters"`
}

// RouteMatch 表示 Envoy route match 的内部模型
type RouteMatch struct {
	PathPrefix string        `json:"pathPrefix"`
	Methods    []string      `json:"methods"`
	Headers    []HeaderMatch `json:"headers"`
}

// HeaderMatch 表示 Envoy header matcher 的内部模型
type HeaderMatch struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// WeightedCluster 表示 Envoy weighted cluster 的内部模型
type WeightedCluster struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// Cluster 表示 Envoy cluster 的内部模型
type Cluster struct {
	Name string `json:"name"`
}

// EndpointAssignment 表示 Envoy endpoint assignment 的内部模型
type EndpointAssignment struct {
	ClusterName string     `json:"clusterName"`
	Endpoints   []Endpoint `json:"endpoints"`
}

// Endpoint 表示 Envoy endpoint 的内部模型
type Endpoint struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// Target 返回运行时 target 名称
func (Translator) Target() string {
	return "xds"
}

// Translate 将逻辑网关模型转换成 xDS 运行时快照
func (t Translator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	config := Config{
		Listeners:           make([]Listener, 0, len(logical.Listeners)),
		RouteConfigs:        make([]RouteConfig, 0, len(logical.Listeners)),
		Clusters:            make([]Cluster, 0, len(logical.Upstreams)),
		EndpointAssignments: make([]EndpointAssignment, 0, len(logical.Upstreams)),
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
						Methods:    slices.Clone(rule.Methods),
					},
					TimeoutMillis:    rule.TimeoutMillis,
					WeightedClusters: make([]WeightedCluster, 0, len(rule.Upstreams)),
				}
				if len(rule.Headers) > 0 {
					xdsRoute.Match.Headers = make([]HeaderMatch, 0, len(rule.Headers))
					for _, header := range rule.Headers {
						xdsRoute.Match.Headers = append(xdsRoute.Match.Headers, HeaderMatch{
							Name:  header.Name,
							Value: header.Value,
						})
					}
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
		config.Clusters = append(config.Clusters, Cluster{
			Name: upstream.Name,
		})

		assignment := EndpointAssignment{
			ClusterName: upstream.Name,
			Endpoints:   make([]Endpoint, 0, len(upstream.Endpoints)),
		}
		for _, endpoint := range upstream.Endpoints {
			assignment.Endpoints = append(assignment.Endpoints, Endpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
			})
		}
		config.EndpointAssignments = append(config.EndpointAssignments, assignment)
	}

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf("xds/%s", logical.Name),
		Config:  config,
	}, nil
}
