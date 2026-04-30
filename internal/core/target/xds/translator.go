// Package xds 将逻辑网关模型翻译成 Envoy-oriented 配置
package xds

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	targetName     string = "xds"
	versionPrefix  string = "xds/%s"
	wildcardDomain string = "*"
)

// Translator 负责生成 xDS target 的配置快照
type Translator struct{}

// Config 表示 xDS target 的内部配置载荷
type Config struct {
	Listeners           []Listener           `json:"listeners"`
	RouteConfigs        []RouteConfig        `json:"routeConfigs"`
	Clusters            []Cluster            `json:"clusters"`
	EndpointAssignments []EndpointAssignment `json:"endpointAssignments"`
	AIRoutes            []AIRoute            `json:"aiRoutes"`
	AIProviders         []AIProvider         `json:"aiProviders"`
	AIModels            []AIModel            `json:"aiModels"`
	AIPolicies          []AIPolicy           `json:"aiPolicies"`
	Plugins             []Plugin             `json:"plugins"`
	PluginBindings      []PluginBinding      `json:"pluginBindings"`
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

// AIRoute 表示 AI 路由的 xDS 内部模型
type AIRoute struct {
	Name       string       `json:"name"`
	Domains    []string     `json:"domains"`
	Match      AIRouteMatch `json:"match"`
	Model      string       `json:"model"`
	Models     []AIModelRef `json:"models"`
	PolicyRefs []string     `json:"policyRefs"`
}

// AIRouteMatch 表示 AI 路由匹配条件
type AIRouteMatch struct {
	Path       string `json:"path"`
	PathPrefix string `json:"pathPrefix"`
}

// AIModelRef 表示 AI 路由中的模型权重
type AIModelRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// AIProvider 表示 AI 模型供应商的 xDS 内部模型
type AIProvider struct {
	Name     string                  `json:"name"`
	Type     resource.AIProviderType `json:"type"`
	Endpoint string                  `json:"endpoint"`
	Models   []string                `json:"models"`
}

// AIModel 表示 AI 模型映射的 xDS 内部模型
type AIModel struct {
	Name          string   `json:"name"`
	ProviderRef   string   `json:"providerRef"`
	ProviderModel string   `json:"providerModel"`
	Capabilities  []string `json:"capabilities"`
}

// AIPolicy 表示 AI 请求策略的 xDS 内部模型
type AIPolicy struct {
	Name            string                         `json:"name"`
	ExecutionTarget resource.AIExecutionTargetType `json:"executionTarget"`
	TimeoutMillis   int                            `json:"timeoutMillis"`
	RetryAttempts   int                            `json:"retryAttempts"`
	FallbackEnabled bool                           `json:"fallbackEnabled"`
	FallbackModels  []string                       `json:"fallbackModels"`
	UsageEnabled    bool                           `json:"usageEnabled"`
}

// Plugin 表示插件的 xDS 内部模型
type Plugin struct {
	Name     string                 `json:"name"`
	Runtime  resource.PluginRuntime `json:"runtime"`
	Version  string                 `json:"version"`
	Endpoint string                 `json:"endpoint"`
	Image    string                 `json:"image"`
}

// PluginBinding 表示插件绑定的 xDS 内部模型
type PluginBinding struct {
	Name          string                       `json:"name"`
	Target        PluginTarget                 `json:"target"`
	Phase         resource.PluginPhase         `json:"phase"`
	Priority      int                          `json:"priority"`
	FailurePolicy resource.PluginFailurePolicy `json:"failurePolicy"`
	Plugins       []PluginRef                  `json:"plugins"`
}

// PluginTarget 表示插件绑定目标
type PluginTarget struct {
	Kind resource.Kind `json:"kind"`
	Name string        `json:"name"`
}

// PluginRef 表示插件引用
type PluginRef struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config,omitempty"`
}

// Target 返回运行时 target 名称
func (Translator) Target() string {
	return targetName
}

// Translate 将逻辑网关模型转换成 xDS 运行时快照
func (t Translator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	config := Config{
		Listeners:           make([]Listener, 0, len(logical.Listeners)),
		RouteConfigs:        make([]RouteConfig, 0, len(logical.Listeners)),
		Clusters:            make([]Cluster, 0, len(logical.Upstreams)),
		EndpointAssignments: make([]EndpointAssignment, 0, len(logical.Upstreams)),
		AIRoutes:            make([]AIRoute, 0, len(logical.AIRoutes)),
		AIProviders:         make([]AIProvider, 0, len(logical.AIProviders)),
		AIModels:            make([]AIModel, 0, len(logical.AIModels)),
		AIPolicies:          make([]AIPolicy, 0, len(logical.AIPolicies)),
		Plugins:             make([]Plugin, 0, len(logical.Plugins)),
		PluginBindings:      make([]PluginBinding, 0, len(logical.PluginBindings)),
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
			domains := slices.Clone(route.Hostnames)
			if len(domains) == 0 {
				domains = []string{wildcardDomain}
			}
			virtualHost := VirtualHost{
				Name:    route.Name,
				Domains: domains,
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

	for _, route := range logical.AIRoutes {
		domains := slices.Clone(route.Hostnames)
		if len(domains) == 0 {
			domains = []string{wildcardDomain}
		}
		xdsRoute := AIRoute{
			Name:    route.Name,
			Domains: domains,
			Match: AIRouteMatch{
				Path:       route.Path,
				PathPrefix: route.PathPrefix,
			},
			Model:      route.Model,
			Models:     make([]AIModelRef, 0, len(route.Models)),
			PolicyRefs: slices.Clone(route.PolicyRefs),
		}
		for _, model := range route.Models {
			xdsRoute.Models = append(xdsRoute.Models, AIModelRef{
				Name:   model.Name,
				Weight: model.Weight,
			})
		}
		config.AIRoutes = append(config.AIRoutes, xdsRoute)
	}

	for _, provider := range logical.AIProviders {
		config.AIProviders = append(config.AIProviders, AIProvider{
			Name:     provider.Name,
			Type:     provider.Type,
			Endpoint: provider.Endpoint,
			Models:   slices.Clone(provider.Models),
		})
	}

	for _, model := range logical.AIModels {
		config.AIModels = append(config.AIModels, AIModel{
			Name:          model.Name,
			ProviderRef:   model.ProviderRef,
			ProviderModel: model.ProviderModel,
			Capabilities:  slices.Clone(model.Capabilities),
		})
	}

	for _, policy := range logical.AIPolicies {
		config.AIPolicies = append(config.AIPolicies, AIPolicy{
			Name:            policy.Name,
			ExecutionTarget: policy.ExecutionTarget,
			TimeoutMillis:   policy.TimeoutMillis,
			RetryAttempts:   policy.RetryAttempts,
			FallbackEnabled: policy.FallbackEnabled,
			FallbackModels:  slices.Clone(policy.FallbackModels),
			UsageEnabled:    policy.UsageEnabled,
		})
	}

	for _, plugin := range logical.Plugins {
		config.Plugins = append(config.Plugins, Plugin{
			Name:     plugin.Name,
			Runtime:  plugin.Runtime,
			Version:  plugin.Version,
			Endpoint: plugin.Endpoint,
			Image:    plugin.Image,
		})
	}

	for _, binding := range logical.PluginBindings {
		xdsBinding := PluginBinding{
			Name: binding.Name,
			Target: PluginTarget{
				Kind: binding.Target.Kind,
				Name: binding.Target.Name,
			},
			Phase:         binding.Phase,
			Priority:      binding.Priority,
			FailurePolicy: binding.FailurePolicy,
			Plugins:       make([]PluginRef, 0, len(binding.Plugins)),
		}
		for _, plugin := range binding.Plugins {
			xdsBinding.Plugins = append(xdsBinding.Plugins, PluginRef{
				Name:   plugin.Name,
				Config: slices.Clone(plugin.Config),
			})
		}
		config.PluginBindings = append(config.PluginBindings, xdsBinding)
	}

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf(versionPrefix, logical.Name),
		Config:  config,
	}, nil
}
