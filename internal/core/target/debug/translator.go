// Package debug 将逻辑网关模型翻译成便于检查的快照
package debug

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
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
	AIModels          []AIModel         `json:"aiModels"`
	AIPolicies        []AIPolicy        `json:"aiPolicies"`
	Plugins           []Plugin          `json:"plugins"`
	AuthPolicies      []AuthPolicy      `json:"authPolicies"`
	RateLimitPolicies []RateLimitPolicy `json:"rateLimitPolicies"`
	RedisStores       []RedisStore      `json:"redisStores"`
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
	Name          string        `json:"name"`
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
	Hostnames  []string        `json:"hostnames"`
	Path       string          `json:"path"`
	PathPrefix string          `json:"pathPrefix"`
	Model      string          `json:"model"`
	Models     []AIModelRef    `json:"models"`
	Providers  []AIProviderRef `json:"providers"`
	PolicyRefs []string        `json:"policyRefs"`
}

// AIModelRef 表示 debug 配置中的 AIModel 引用
type AIModelRef struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
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

// AIModel 表示 debug 配置中的 AI 模型映射
type AIModel struct {
	Name          string   `json:"name"`
	ProviderRef   string   `json:"providerRef"`
	ProviderModel string   `json:"providerModel"`
	Capabilities  []string `json:"capabilities"`
}

// AIPolicy 表示 debug 配置中的 AI 请求策略
type AIPolicy struct {
	Name            string                         `json:"name"`
	ExecutionTarget resource.AIExecutionTargetType `json:"executionTarget"`
	TimeoutMillis   int                            `json:"timeoutMillis"`
	RetryAttempts   int                            `json:"retryAttempts"`
	FallbackEnabled bool                           `json:"fallbackEnabled"`
	FallbackModels  []string                       `json:"fallbackModels"`
	UsageEnabled    bool                           `json:"usageEnabled"`
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
	Name          string                          `json:"name"`
	DisplayName   string                          `json:"displayName"`
	Mode          resource.RateLimitMode          `json:"mode"`
	Rules         []RateLimitRule                 `json:"rules"`
	Global        *GlobalRateLimit                `json:"global,omitempty"`
	Response      RateLimitResponse               `json:"response"`
	FailurePolicy resource.RateLimitFailurePolicy `json:"failurePolicy"`
}

// RateLimitRule 表示 debug 配置中的单条限流规则
type RateLimitRule struct {
	Name      string                      `json:"name"`
	Key       []RateLimitKeyPart          `json:"key"`
	Limit     RateLimitQuota              `json:"limit"`
	Algorithm resource.RateLimitAlgorithm `json:"algorithm"`
}

// RateLimitKeyPart 表示 debug 配置中的限流 key 组成部分
type RateLimitKeyPart struct {
	Type resource.RateLimitKeyType `json:"type"`
	Name string                    `json:"name,omitempty"`
}

// RateLimitQuota 表示 debug 配置中的限流额度
type RateLimitQuota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
}

// GlobalRateLimit 表示 debug 配置中的 Redis-backed global limit
type GlobalRateLimit struct {
	RedisRef      string `json:"redisRef"`
	Prefix        string `json:"prefix,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis,omitempty"`
}

// RateLimitResponse 表示 debug 配置中的超限响应
type RateLimitResponse struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

// RedisStore 表示 debug 配置中的 Redis 连接配置
type RedisStore struct {
	Name                 string             `json:"name"`
	DisplayName          string             `json:"displayName"`
	Mode                 resource.RedisMode `json:"mode"`
	Address              string             `json:"address"`
	Addresses            []string           `json:"addresses,omitempty"`
	DB                   int                `json:"db,omitempty"`
	TLS                  bool               `json:"tls,omitempty"`
	TLSServerName        string             `json:"tlsServerName,omitempty"`
	Username             string             `json:"username,omitempty"`
	PasswordRef          string             `json:"passwordRef,omitempty"`
	ConnectTimeoutMillis int                `json:"connectTimeoutMillis,omitempty"`
	CommandTimeoutMillis int                `json:"commandTimeoutMillis,omitempty"`
	PoolSize             int                `json:"poolSize,omitempty"`
	MinIdleConns         int                `json:"minIdleConns,omitempty"`
	SentinelMaster       string             `json:"sentinelMaster,omitempty"`
}

// PolicyBinding 表示 debug 配置中的策略绑定
type PolicyBinding struct {
	Name     string       `json:"name"`
	Target   PolicyTarget `json:"target"`
	Policies []PolicyRef  `json:"policies"`
}

// PolicyTarget 表示 debug 配置中的策略绑定目标
type PolicyTarget struct {
	Kind     resource.Kind `json:"kind"`
	Name     string        `json:"name"`
	RuleName string        `json:"ruleName,omitempty"`
}

// PolicyRef 表示 debug 配置中的策略引用
type PolicyRef struct {
	Kind resource.Kind `json:"kind"`
	Name string        `json:"name"`
}

// PluginBinding 表示 debug 配置中的插件绑定
type PluginBinding struct {
	Name          string                       `json:"name"`
	Target        PluginTarget                 `json:"target"`
	Phase         resource.PluginPhase         `json:"phase"`
	Priority      int                          `json:"priority"`
	FailurePolicy resource.PluginFailurePolicy `json:"failurePolicy"`
	Plugins       []PluginRef                  `json:"plugins"`
}

// PluginTarget 表示 debug 配置中的插件绑定目标
type PluginTarget struct {
	Kind resource.Kind `json:"kind"`
	Name string        `json:"name"`
}

// PluginRef 表示 debug 配置中的插件引用
type PluginRef struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config,omitempty"`
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
		AIModels:          make([]AIModel, 0, len(logical.AIModels)),
		AIPolicies:        make([]AIPolicy, 0, len(logical.AIPolicies)),
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
				Name:          rule.Name,
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
			Hostnames:  slices.Clone(route.Hostnames),
			Path:       route.Path,
			PathPrefix: route.PathPrefix,
			Model:      route.Model,
			Models:     make([]AIModelRef, 0, len(route.Models)),
			Providers:  make([]AIProviderRef, 0, len(route.Providers)),
			PolicyRefs: slices.Clone(route.PolicyRefs),
		}
		for _, model := range route.Models {
			debugRoute.Models = append(debugRoute.Models, AIModelRef{
				Name:   model.Name,
				Weight: model.Weight,
			})
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
		config.RateLimitPolicies = append(config.RateLimitPolicies, newRateLimitPolicy(policy))
	}
	for _, store := range logical.RedisStores {
		config.RedisStores = append(config.RedisStores, RedisStore{
			Name:                 store.Name,
			DisplayName:          store.DisplayName,
			Mode:                 store.Mode,
			Address:              store.Address,
			Addresses:            slices.Clone(store.Addresses),
			DB:                   store.DB,
			TLS:                  store.TLS,
			TLSServerName:        store.TLSServerName,
			Username:             store.Username,
			PasswordRef:          store.PasswordRef,
			ConnectTimeoutMillis: store.ConnectTimeoutMillis,
			CommandTimeoutMillis: store.CommandTimeoutMillis,
			PoolSize:             store.PoolSize,
			MinIdleConns:         store.MinIdleConns,
			SentinelMaster:       store.SentinelMaster,
		})
	}
	for _, binding := range logical.PolicyBindings {
		debugBinding := PolicyBinding{
			Name: binding.Name,
			Target: PolicyTarget{
				Kind:     binding.Target.Kind,
				Name:     binding.Target.Name,
				RuleName: binding.Target.RuleName,
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
			Phase:         binding.Phase,
			Priority:      binding.Priority,
			FailurePolicy: binding.FailurePolicy,
			Plugins:       make([]PluginRef, 0, len(binding.Plugins)),
		}
		for _, plugin := range binding.Plugins {
			debugBinding.Plugins = append(debugBinding.Plugins, PluginRef{
				Name:   plugin.Name,
				Config: slices.Clone(plugin.Config),
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

func newRateLimitPolicy(policy ir.LogicalRateLimitPolicy) RateLimitPolicy {
	debugPolicy := RateLimitPolicy{
		Name:          policy.Name,
		DisplayName:   policy.DisplayName,
		Mode:          policy.Mode,
		Rules:         make([]RateLimitRule, 0, len(policy.Rules)),
		Response:      RateLimitResponse(policy.Response),
		FailurePolicy: policy.FailurePolicy,
	}
	for _, rule := range policy.Rules {
		debugPolicy.Rules = append(debugPolicy.Rules, RateLimitRule{
			Name:      rule.Name,
			Key:       newRateLimitKey(rule.Key),
			Limit:     RateLimitQuota(rule.Limit),
			Algorithm: rule.Algorithm,
		})
	}
	if policy.Global != nil {
		debugPolicy.Global = &GlobalRateLimit{
			RedisRef:      policy.Global.RedisRef,
			Prefix:        policy.Global.Prefix,
			TimeoutMillis: policy.Global.TimeoutMillis,
		}
	}
	return debugPolicy
}

func newRateLimitKey(parts []ir.LogicalRateLimitKeyPart) []RateLimitKeyPart {
	result := make([]RateLimitKeyPart, 0, len(parts))
	for _, part := range parts {
		result = append(result, RateLimitKeyPart{
			Type: part.Type,
			Name: part.Name,
		})
	}
	return result
}
