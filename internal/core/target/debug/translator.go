// Package debug 将逻辑网关模型翻译成便于检查的快照
package debug

import (
	"fmt"
	"maps"
	"slices"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/resource"
	"github.com/lgc202/ingate-next/internal/core/runtime"
)

const (
	targetName    string = "debug"
	versionPrefix string = "debug/%s"
)

// Translator 负责生成 debug target 的配置快照
type Translator struct{}

// Config 表示 debug target 的配置载荷
type Config struct {
	Listeners         []Listener        `json:"listeners"`
	Routes            []Route           `json:"routes"`
	AIRoutes          []AIRoute         `json:"aiRoutes"`
	Upstreams         []Upstream        `json:"upstreams"`
	AIProviders       []AIProvider      `json:"aiProviders"`
	Plugins           []Plugin          `json:"plugins"`
	AuthPolicies      []AuthPolicy      `json:"authPolicies"`
	RateLimitPolicies []RateLimitPolicy `json:"rateLimitPolicies"`
	PolicyBindings    []PolicyBinding   `json:"policyBindings"`
	PluginBindings    []PluginBinding   `json:"pluginBindings"`
}

// Listener 表示 debug 配置中的监听器
type Listener struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	Hostname string `json:"hostname"`
}

// Route 表示 debug 配置中的路由
type Route struct {
	Name      string      `json:"name"`
	Hostnames []string    `json:"hostnames"`
	Rules     []RouteRule `json:"rules"`
}

// RouteRule 表示 debug 配置中的路由规则
type RouteRule struct {
	PathPrefix    string        `json:"pathPrefix"`
	Methods       []string      `json:"methods"`
	TimeoutMillis int           `json:"timeoutMillis"`
	Headers       []HeaderMatch `json:"headers"`
	Upstreams     []UpstreamRef `json:"upstreams"`
}

// HeaderMatch 表示 debug 配置中的 HTTP header 精确匹配条件
type HeaderMatch struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// UpstreamRef 表示 debug 配置中的 Upstream 引用
type UpstreamRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// AIRoute 表示 debug 配置中的 AI 路由
type AIRoute struct {
	Name       string          `json:"name"`
	PathPrefix string          `json:"pathPrefix"`
	Model      string          `json:"model"`
	Providers  []AIProviderRef `json:"providers"`
}

// AIProviderRef 表示 debug 配置中的 AIProvider 引用
type AIProviderRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// Upstream 表示 debug 配置中的上游服务
type Upstream struct {
	Name      string     `json:"name"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint 表示 debug 配置中的上游端点
type Endpoint struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// AIProvider 表示 debug 配置中的 AI 模型供应商
type AIProvider struct {
	Name     string                  `json:"name"`
	Type     resource.AIProviderType `json:"type"`
	Endpoint string                  `json:"endpoint"`
	Models   []string                `json:"models"`
}

// Plugin 表示 debug 配置中的插件声明
type Plugin struct {
	Name     string                 `json:"name"`
	Runtime  resource.PluginRuntime `json:"runtime"`
	Version  string                 `json:"version"`
	Endpoint string                 `json:"endpoint"`
	Image    string                 `json:"image"`
}

// AuthPolicy 表示 debug 配置中的认证策略
type AuthPolicy struct {
	Name   string            `json:"name"`
	Type   resource.AuthType `json:"type"`
	APIKey APIKeyAuth        `json:"apiKey"`
}

// APIKeyAuth 表示 debug 配置中的 API Key 认证配置
type APIKeyAuth struct {
	Header string `json:"header"`
	Query  string `json:"query"`
}

// RateLimitPolicy 表示 debug 配置中的限流策略
type RateLimitPolicy struct {
	Name          string                `json:"name"`
	Requests      int                   `json:"requests"`
	WindowSeconds int                   `json:"windowSeconds"`
	KeyBy         resource.RateLimitKey `json:"keyBy"`
	Header        string                `json:"header"`
}

// PolicyBinding 表示 debug 配置中的策略绑定
type PolicyBinding struct {
	Name     string       `json:"name"`
	Target   PolicyTarget `json:"target"`
	Policies []PolicyRef  `json:"policies"`
}

// PolicyTarget 表示 debug 配置中的策略绑定目标
type PolicyTarget struct {
	Kind resource.Kind `json:"kind"`
	Name string        `json:"name"`
}

// PolicyRef 表示 debug 配置中的策略引用
type PolicyRef struct {
	Kind resource.Kind `json:"kind"`
	Name string        `json:"name"`
}

// PluginBinding 表示 debug 配置中的插件绑定
type PluginBinding struct {
	Name    string       `json:"name"`
	Target  PluginTarget `json:"target"`
	Plugins []PluginRef  `json:"plugins"`
}

// PluginTarget 表示 debug 配置中的插件绑定目标
type PluginTarget struct {
	Kind resource.Kind `json:"kind"`
	Name string        `json:"name"`
}

// PluginRef 表示 debug 配置中的插件引用
type PluginRef struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

// Target 返回运行时 target 名称
func (Translator) Target() string {
	return targetName
}

// Translate 将逻辑网关模型转换成 debug 运行时快照
func (t Translator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	config := Config{
		Listeners:         make([]Listener, 0, len(logical.Listeners)),
		Routes:            make([]Route, 0, len(logical.Routes)),
		AIRoutes:          make([]AIRoute, 0, len(logical.AIRoutes)),
		Upstreams:         make([]Upstream, 0, len(logical.Upstreams)),
		AIProviders:       make([]AIProvider, 0, len(logical.AIProviders)),
		Plugins:           make([]Plugin, 0, len(logical.Plugins)),
		AuthPolicies:      make([]AuthPolicy, 0, len(logical.AuthPolicies)),
		RateLimitPolicies: make([]RateLimitPolicy, 0, len(logical.RateLimitPolicies)),
		PolicyBindings:    make([]PolicyBinding, 0, len(logical.PolicyBindings)),
		PluginBindings:    make([]PluginBinding, 0, len(logical.PluginBindings)),
	}

	for _, listener := range logical.Listeners {
		config.Listeners = append(config.Listeners, Listener{
			Name:     listener.Name,
			Protocol: listener.Protocol,
			Port:     listener.Port,
			Hostname: listener.Hostname,
		})
	}
	for _, route := range logical.Routes {
		debugRoute := Route{
			Name:      route.Name,
			Hostnames: slices.Clone(route.Hostnames),
			Rules:     make([]RouteRule, 0, len(route.Rules)),
		}
		for _, rule := range route.Rules {
			debugRule := RouteRule{
				PathPrefix:    rule.PathPrefix,
				Methods:       slices.Clone(rule.Methods),
				TimeoutMillis: rule.TimeoutMillis,
				Upstreams:     make([]UpstreamRef, 0, len(rule.Upstreams)),
			}
			if len(rule.Headers) > 0 {
				debugRule.Headers = make([]HeaderMatch, 0, len(rule.Headers))
				for _, header := range rule.Headers {
					debugRule.Headers = append(debugRule.Headers, HeaderMatch{
						Name:  header.Name,
						Value: header.Value,
					})
				}
			}
			for _, upstream := range rule.Upstreams {
				debugRule.Upstreams = append(debugRule.Upstreams, UpstreamRef{
					Name:   upstream.Name,
					Weight: upstream.Weight,
				})
			}
			debugRoute.Rules = append(debugRoute.Rules, debugRule)
		}
		config.Routes = append(config.Routes, debugRoute)
	}
	for _, route := range logical.AIRoutes {
		debugRoute := AIRoute{
			Name:       route.Name,
			PathPrefix: route.PathPrefix,
			Model:      route.Model,
			Providers:  make([]AIProviderRef, 0, len(route.Providers)),
		}
		for _, provider := range route.Providers {
			debugRoute.Providers = append(debugRoute.Providers, AIProviderRef{
				Name:   provider.Name,
				Weight: provider.Weight,
			})
		}
		config.AIRoutes = append(config.AIRoutes, debugRoute)
	}
	for _, upstream := range logical.Upstreams {
		debugUpstream := Upstream{
			Name:      upstream.Name,
			Endpoints: make([]Endpoint, 0, len(upstream.Endpoints)),
		}
		for _, endpoint := range upstream.Endpoints {
			debugUpstream.Endpoints = append(debugUpstream.Endpoints, Endpoint{
				Address: endpoint.Address,
				Port:    endpoint.Port,
			})
		}
		config.Upstreams = append(config.Upstreams, debugUpstream)
	}
	for _, provider := range logical.AIProviders {
		config.AIProviders = append(config.AIProviders, AIProvider{
			Name:     provider.Name,
			Type:     provider.Type,
			Endpoint: provider.Endpoint,
			Models:   slices.Clone(provider.Models),
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
	for _, policy := range logical.AuthPolicies {
		config.AuthPolicies = append(config.AuthPolicies, AuthPolicy{
			Name: policy.Name,
			Type: policy.Type,
			APIKey: APIKeyAuth{
				Header: policy.APIKey.Header,
				Query:  policy.APIKey.Query,
			},
		})
	}
	for _, policy := range logical.RateLimitPolicies {
		config.RateLimitPolicies = append(config.RateLimitPolicies, RateLimitPolicy{
			Name:          policy.Name,
			Requests:      policy.Requests,
			WindowSeconds: policy.WindowSeconds,
			KeyBy:         policy.KeyBy,
			Header:        policy.Header,
		})
	}
	for _, binding := range logical.PolicyBindings {
		debugBinding := PolicyBinding{
			Name: binding.Name,
			Target: PolicyTarget{
				Kind: binding.Target.Kind,
				Name: binding.Target.Name,
			},
			Policies: make([]PolicyRef, 0, len(binding.Policies)),
		}
		for _, policy := range binding.Policies {
			debugBinding.Policies = append(debugBinding.Policies, PolicyRef{
				Kind: policy.Kind,
				Name: policy.Name,
			})
		}
		config.PolicyBindings = append(config.PolicyBindings, debugBinding)
	}
	for _, binding := range logical.PluginBindings {
		debugBinding := PluginBinding{
			Name: binding.Name,
			Target: PluginTarget{
				Kind: binding.Target.Kind,
				Name: binding.Target.Name,
			},
			Plugins: make([]PluginRef, 0, len(binding.Plugins)),
		}
		for _, plugin := range binding.Plugins {
			debugBinding.Plugins = append(debugBinding.Plugins, PluginRef{
				Name:   plugin.Name,
				Config: maps.Clone(plugin.Config),
			})
		}
		config.PluginBindings = append(config.PluginBindings, debugBinding)
	}

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf(versionPrefix, logical.Name),
		Config:  config,
	}, nil
}
