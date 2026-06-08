// Package xds 将逻辑网关模型翻译成 Envoy-oriented 配置
package xds

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	targetName                            string = "xds"
	versionPrefix                         string = "xds/%s"
	wildcardDomain                        string = "*"
	wildcardModel                         string = "*"
	aiProviderClusterNameFormat           string = "ai-provider/%s"
	httpScheme                            string = "http"
	httpsScheme                           string = "https"
	pluginConfigRulesKey                  string = "_rules_"
	pluginMatchRouteKey                   string = "_match_route_"
	pluginMatchDomainKey                  string = "_match_domain_"
	pluginProviderKey                     string = "provider"
	pluginModelKey                        string = "model"
	pluginModelsKey                       string = "models"
	pluginPolicyRefsKey                   string = "policyRefs"
	pluginProviderNameKey                 string = "name"
	pluginProviderTypeKey                 string = "type"
	pluginProviderURLKey                  string = "endpoint"
	pluginModelMappingKey                 string = "modelMapping"
	rateLimitExecutorCluster              string = "ingate-rate-limit-executor"
	rateLimitExecutorAddress              string = "127.0.0.1"
	rateLimitExecutorPath                 string = "/v1/rate-limit/check"
	defaultHTTPPort                       int    = 80
	defaultHTTPSPort                      int    = 443
	defaultRateLimitExecutorPort          int    = 18081
	defaultRateLimitExecutorTimeoutMillis int    = 50
)

// ClusterDiscoveryType 表示 Envoy cluster 服务发现方式
type ClusterDiscoveryType string

const (
	// ClusterDiscoveryTypeEDS 表示通过 EDS 获取端点
	ClusterDiscoveryTypeEDS ClusterDiscoveryType = "EDS"
	// ClusterDiscoveryTypeLogicalDNS 表示通过 DNS 解析上游地址
	ClusterDiscoveryTypeLogicalDNS ClusterDiscoveryType = "LOGICAL_DNS"
)

// Translator 负责生成 xDS target 的配置快照
type Translator struct{}

// Config 表示 xDS target 的内部配置载荷
type Config struct {
	GatewayName         string               `json:"gatewayName"`
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
	ManagedRateLimit    *ManagedRateLimit    `json:"managedRateLimit,omitempty"`
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
	GatewayName            string            `json:"gatewayName"`
	Name                   string            `json:"name"`
	RuleName               string            `json:"ruleName,omitempty"`
	Match                  RouteMatch        `json:"match"`
	TimeoutMillis          int               `json:"timeoutMillis"`
	RequestHeadersToAdd    []HeaderValue     `json:"requestHeadersToAdd,omitempty"`
	RequestHeadersToRemove []string          `json:"requestHeadersToRemove,omitempty"`
	RetryPolicy            *RetryPolicy      `json:"retryPolicy,omitempty"`
	WeightedClusters       []WeightedCluster `json:"weightedClusters"`
}

// RouteMatch 表示 Envoy route match 的内部模型
type RouteMatch struct {
	Path       string        `json:"path"`
	PathPrefix string        `json:"pathPrefix"`
	Methods    []string      `json:"methods"`
	Headers    []HeaderMatch `json:"headers"`
}

// HeaderMatch 表示 Envoy header matcher 的内部模型
type HeaderMatch struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HeaderValue 表示 Envoy 请求 header 写入动作的内部模型
type HeaderValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RetryPolicy 表示 Envoy route retry policy 的内部模型
type RetryPolicy struct {
	Attempts            int `json:"attempts,omitempty"`
	PerTryTimeoutMillis int `json:"perTryTimeoutMillis,omitempty"`
}

// WeightedCluster 表示 Envoy weighted cluster 的内部模型
type WeightedCluster struct {
	Name   string `json:"name"`
	Weight int    `json:"weight"`
}

// Cluster 表示 Envoy cluster 的内部模型
type Cluster struct {
	Name          string               `json:"name"`
	DiscoveryType ClusterDiscoveryType `json:"discoveryType,omitempty"`
	Address       string               `json:"address,omitempty"`
	Port          int                  `json:"port,omitempty"`
	TLS           bool                 `json:"tls,omitempty"`
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
	Name       string          `json:"name"`
	Domains    []string        `json:"domains"`
	Match      AIRouteMatch    `json:"match"`
	Model      string          `json:"model"`
	Models     []AIModelRef    `json:"models"`
	Providers  []AIProviderRef `json:"providers"`
	PolicyRefs []string        `json:"policyRefs"`
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

// AIProviderRef 表示 AI 路由中的供应商权重
type AIProviderRef struct {
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

// ManagedRateLimit 表示系统内置限流插件的可执行配置
type ManagedRateLimit struct {
	Bindings    []RateLimitBinding    `json:"bindings"`
	RedisStores []RateLimitRedisStore `json:"redisStores,omitempty"`
	Executor    *RateLimitExecutor    `json:"executor,omitempty"`
}

// RateLimitExecutor 表示内置限流执行器的运行时入口
type RateLimitExecutor struct {
	ClusterName   string `json:"clusterName"`
	Path          string `json:"path"`
	TimeoutMillis int    `json:"timeoutMillis"`
}

// RateLimitBinding 表示限流策略绑定展开后的执行配置
type RateLimitBinding struct {
	Name     string            `json:"name"`
	Target   RateLimitTarget   `json:"target"`
	Policies []RateLimitPolicy `json:"policies"`
}

// RateLimitTarget 表示限流执行目标
type RateLimitTarget struct {
	Kind     resource.Kind `json:"kind"`
	Name     string        `json:"name"`
	RuleName string        `json:"ruleName,omitempty"`
}

// RateLimitPolicy 表示内置限流插件消费的策略配置
type RateLimitPolicy struct {
	Name          string                          `json:"name"`
	DisplayName   string                          `json:"displayName"`
	Mode          resource.RateLimitMode          `json:"mode"`
	Rules         []RateLimitRule                 `json:"rules"`
	Global        *GlobalRateLimit                `json:"global,omitempty"`
	Response      RateLimitResponse               `json:"response"`
	FailurePolicy resource.RateLimitFailurePolicy `json:"failurePolicy"`
}

// RateLimitRule 表示一条限流规则
type RateLimitRule struct {
	Name      string                      `json:"name"`
	Key       []RateLimitKeyPart          `json:"key"`
	Limit     RateLimitQuota              `json:"limit"`
	Algorithm resource.RateLimitAlgorithm `json:"algorithm"`
}

// RateLimitKeyPart 表示限流 key 的组成部分
type RateLimitKeyPart struct {
	Type resource.RateLimitKeyType `json:"type"`
	Name string                    `json:"name,omitempty"`
}

// RateLimitQuota 表示限流额度
type RateLimitQuota struct {
	Requests      int `json:"requests"`
	WindowSeconds int `json:"windowSeconds"`
	Burst         int `json:"burst,omitempty"`
}

// GlobalRateLimit 表示 Redis-backed global limit 配置
type GlobalRateLimit struct {
	RedisRef      string `json:"redisRef"`
	Prefix        string `json:"prefix,omitempty"`
	TimeoutMillis int    `json:"timeoutMillis,omitempty"`
}

// RateLimitResponse 表示超限响应配置
type RateLimitResponse struct {
	StatusCode         int    `json:"statusCode,omitempty"`
	Message            string `json:"message,omitempty"`
	QuotaHeaderEnabled bool   `json:"quotaHeaderEnabled,omitempty"`
}

// RateLimitRedisStore 表示内置限流插件使用的 Redis 连接配置
type RateLimitRedisStore struct {
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

type pluginBindingGroupKey struct {
	pluginName    string
	phase         resource.PluginPhase
	priority      int
	failurePolicy resource.PluginFailurePolicy
}

type pluginBindingGroup struct {
	pluginName string
	binding    PluginBinding
	rules      []map[string]any
}

// Target 返回运行时 target 名称
func (Translator) Target() string {
	return targetName
}

// Translate 将逻辑网关模型转换成 xDS 运行时快照
func (t Translator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	config := Config{
		GatewayName:         logical.Name,
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
					GatewayName: logical.Name,
					Name:        route.Name,
					RuleName:    rule.Name,
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
				if len(rule.RequestHeadersToAdd) > 0 {
					xdsRoute.RequestHeadersToAdd = make([]HeaderValue, 0, len(rule.RequestHeadersToAdd))
					for _, header := range rule.RequestHeadersToAdd {
						xdsRoute.RequestHeadersToAdd = append(xdsRoute.RequestHeadersToAdd, HeaderValue{
							Name:  header.Name,
							Value: header.Value,
						})
					}
				}
				xdsRoute.RequestHeadersToRemove = slices.Clone(rule.RequestHeadersToRemove)
				if rule.Retry.Attempts > 0 {
					xdsRoute.RetryPolicy = &RetryPolicy{
						Attempts:            rule.Retry.Attempts,
						PerTryTimeoutMillis: rule.Retry.PerTryTimeoutMillis,
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
			Name:          upstream.Name,
			DiscoveryType: ClusterDiscoveryTypeEDS,
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
			Providers:  make([]AIProviderRef, 0, len(route.Providers)),
			PolicyRefs: slices.Clone(route.PolicyRefs),
		}
		for _, model := range route.Models {
			xdsRoute.Models = append(xdsRoute.Models, AIModelRef{
				Name:   model.Name,
				Weight: model.Weight,
			})
		}
		for _, provider := range route.Providers {
			xdsRoute.Providers = append(xdsRoute.Providers, AIProviderRef{
				Name:   provider.Name,
				Weight: provider.Weight,
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

	// 普通 Route 只需要生成 RDS route，并指向用户声明的 Upstream
	// AIRoute 额外需要把 AIProvider endpoint 变成真实 Envoy cluster
	// 这样请求路径在数据面上是真实可达的，而不是只停留在插件配置里
	if err := t.appendAIRouteRuntime(&config); err != nil {
		return runtime.RuntimeSnapshot{}, err
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

	// AIRoute 的模型、供应商和策略不会直接进入 Listener
	// 它们会被编译进 Wasm 插件的 _rules_，插件按 route/domain 匹配后读取这些配置
	pluginBindings, err := t.translatePluginBindings(logical, config)
	if err != nil {
		return runtime.RuntimeSnapshot{}, err
	}
	config.PluginBindings = pluginBindings
	config.ManagedRateLimit = t.translateManagedRateLimit(logical)
	if config.ManagedRateLimit != nil && config.ManagedRateLimit.Executor != nil {
		config.Clusters = append(config.Clusters, Cluster{
			Name:          config.ManagedRateLimit.Executor.ClusterName,
			DiscoveryType: ClusterDiscoveryTypeLogicalDNS,
			Address:       rateLimitExecutorAddress,
			Port:          defaultRateLimitExecutorPort,
		})
	}

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf(versionPrefix, logical.Name),
		Config:  config,
	}, nil
}

func (t Translator) translateManagedRateLimit(logical ir.LogicalGateway) *ManagedRateLimit {
	if len(logical.RateLimitPolicies) == 0 || len(logical.PolicyBindings) == 0 {
		return nil
	}

	policiesByName := make(map[string]ir.LogicalRateLimitPolicy, len(logical.RateLimitPolicies))
	hasGlobalPolicy := false
	for _, policy := range logical.RateLimitPolicies {
		policiesByName[policy.Name] = policy
		if policy.Mode == resource.RateLimitModeGlobal {
			hasGlobalPolicy = true
		}
	}

	config := &ManagedRateLimit{
		Bindings:    make([]RateLimitBinding, 0, len(logical.PolicyBindings)),
		RedisStores: make([]RateLimitRedisStore, 0, len(logical.RedisStores)),
	}
	for _, binding := range logical.PolicyBindings {
		rateLimitBinding := RateLimitBinding{
			Name: binding.Name,
			Target: RateLimitTarget{
				Kind:     binding.Target.Kind,
				Name:     binding.Target.Name,
				RuleName: binding.Target.RuleName,
			},
			Policies: make([]RateLimitPolicy, 0, len(binding.Policies)),
		}
		for _, policyRef := range binding.Policies {
			if policyRef.Kind != resource.KindRateLimitPolicy {
				continue
			}
			policy, ok := policiesByName[policyRef.Name]
			if !ok {
				continue
			}
			rateLimitBinding.Policies = append(rateLimitBinding.Policies, t.rateLimitPolicy(policy))
		}
		if len(rateLimitBinding.Policies) == 0 {
			continue
		}
		config.Bindings = append(config.Bindings, rateLimitBinding)
	}
	for _, store := range logical.RedisStores {
		config.RedisStores = append(config.RedisStores, RateLimitRedisStore{
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
	if len(config.Bindings) == 0 {
		return nil
	}
	if hasGlobalPolicy {
		config.Executor = &RateLimitExecutor{
			ClusterName:   rateLimitExecutorCluster,
			Path:          rateLimitExecutorPath,
			TimeoutMillis: defaultRateLimitExecutorTimeoutMillis,
		}
	}
	return config
}

func (t Translator) rateLimitPolicy(policy ir.LogicalRateLimitPolicy) RateLimitPolicy {
	result := RateLimitPolicy{
		Name:          policy.Name,
		DisplayName:   policy.DisplayName,
		Mode:          policy.Mode,
		Rules:         make([]RateLimitRule, 0, len(policy.Rules)),
		Response:      RateLimitResponse(policy.Response),
		FailurePolicy: policy.FailurePolicy,
	}
	for _, rule := range policy.Rules {
		result.Rules = append(result.Rules, RateLimitRule{
			Name:      rule.Name,
			Key:       t.rateLimitKey(rule.Key),
			Limit:     RateLimitQuota(rule.Limit),
			Algorithm: rule.Algorithm,
		})
	}
	if policy.Global != nil {
		result.Global = &GlobalRateLimit{
			RedisRef:      policy.Global.RedisRef,
			Prefix:        policy.Global.Prefix,
			TimeoutMillis: policy.Global.TimeoutMillis,
		}
	}
	return result
}

func (t Translator) rateLimitKey(parts []ir.LogicalRateLimitKeyPart) []RateLimitKeyPart {
	result := make([]RateLimitKeyPart, 0, len(parts))
	for _, part := range parts {
		result = append(result, RateLimitKeyPart{
			Type: part.Type,
			Name: part.Name,
		})
	}
	return result
}

func (t Translator) appendAIRouteRuntime(config *Config) error {
	aiProvidersByName := t.aiProvidersByName(config.AIProviders)
	aiModelsByName := t.aiModelsByName(config.AIModels)
	clustersByName := make(map[string]bool, len(config.Clusters))
	for _, cluster := range config.Clusters {
		clustersByName[cluster.Name] = true
	}

	for _, aiRoute := range config.AIRoutes {
		// 第一阶段只选一个执行目标：优先取 AIRoute 的第一个 AIModel，再取直接 ProviderRef
		// 后续做模型权重、fallback、多 provider 时，再把这里扩展成更完整的 provider router
		provider, ok := t.aiRouteProvider(aiRoute, aiProvidersByName, aiModelsByName)
		if !ok {
			continue
		}
		clusterName := fmt.Sprintf(aiProviderClusterNameFormat, provider.Name)
		if !clustersByName[clusterName] {
			cluster, err := t.aiProviderCluster(provider)
			if err != nil {
				return err
			}
			config.Clusters = append(config.Clusters, cluster)
			clustersByName[clusterName] = true
		}
		t.appendAIRouteToRouteConfigs(config, aiRoute, clusterName)
	}
	return nil
}

func (t Translator) appendAIRouteToRouteConfigs(config *Config, aiRoute AIRoute, clusterName string) {
	for i := range config.RouteConfigs {
		// AI route 作为独立 virtual host 追加到每个 listener 的 RDS 配置
		// Envoy 负责按 host/path 把请求送到 provider cluster，Wasm 插件负责改写协议、注入模型和处理策略
		config.RouteConfigs[i].VirtualHosts = append(config.RouteConfigs[i].VirtualHosts, VirtualHost{
			Name:    aiRoute.Name,
			Domains: slices.Clone(aiRoute.Domains),
			Routes: []Route{
				{
					GatewayName: config.GatewayName,
					Name:        aiRoute.Name,
					Match: RouteMatch{
						Path:       aiRoute.Match.Path,
						PathPrefix: aiRoute.Match.PathPrefix,
					},
					WeightedClusters: []WeightedCluster{
						{Name: clusterName, Weight: 100},
					},
				},
			},
		})
	}
}

func (t Translator) translatePluginBindings(logical ir.LogicalGateway, config Config) ([]PluginBinding, error) {
	aiRoutesByName := t.aiRoutesByName(config.AIRoutes)
	aiProvidersByName := t.aiProvidersByName(config.AIProviders)
	aiModelsByName := t.aiModelsByName(config.AIModels)

	bindings := make([]PluginBinding, 0, len(logical.PluginBindings))
	groups := make(map[pluginBindingGroupKey]int)
	var aiGroups []pluginBindingGroup
	for _, binding := range logical.PluginBindings {
		if binding.Target.Kind != resource.KindAIRoute {
			bindings = append(bindings, t.directPluginBinding(binding))
			continue
		}

		// Higress-like 做法：AIRoute 绑定不会变成一个路由专属 Envoy filter
		// 同一个 Wasm 插件只在 HTTP filter chain 加载一次，多个 AIRoute 合并成 _rules_
		// 插件运行时根据 _match_route_ / _match_domain_ 判断当前请求是否应用某条规则
		aiRoute, ok := aiRoutesByName[binding.Target.Name]
		if !ok {
			continue
		}
		for _, plugin := range binding.Plugins {
			key := pluginBindingGroupKey{
				pluginName:    plugin.Name,
				phase:         binding.Phase,
				priority:      binding.Priority,
				failurePolicy: binding.FailurePolicy,
			}
			index, ok := groups[key]
			if !ok {
				index = len(aiGroups)
				groups[key] = index
				aiGroups = append(aiGroups, pluginBindingGroup{
					pluginName: plugin.Name,
					binding: PluginBinding{
						Name: binding.Name,
						Target: PluginTarget{
							Kind: resource.KindGateway,
							Name: logical.Name,
						},
						Phase:         binding.Phase,
						Priority:      binding.Priority,
						FailurePolicy: binding.FailurePolicy,
					},
				})
			}

			rule, err := t.buildAIPluginRule(aiRoute, plugin, aiProvidersByName, aiModelsByName)
			if err != nil {
				return nil, err
			}
			aiGroups[index].rules = append(aiGroups[index].rules, rule)
		}
	}

	for _, group := range aiGroups {
		config, err := json.Marshal(map[string]any{
			pluginConfigRulesKey: group.rules,
		})
		if err != nil {
			return nil, err
		}
		group.binding.Plugins = []PluginRef{
			{
				Name:   group.pluginName,
				Config: config,
			},
		}
		bindings = append(bindings, group.binding)
	}
	return bindings, nil
}

func (t Translator) directPluginBinding(binding ir.LogicalPluginBinding) PluginBinding {
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
	return xdsBinding
}

func (t Translator) buildAIPluginRule(
	route AIRoute,
	plugin ir.LogicalPluginRef,
	providers map[string]AIProvider,
	models map[string]AIModel,
) (map[string]any, error) {
	rule := make(map[string]any)
	if len(plugin.Config) > 0 {
		// PluginRef.Config 只作为补充配置进入 rule
		// provider/model/policy 仍然由 AIProvider、AIModel、AIPolicy 这些一等资源生成，避免用户手写大块 JSON
		if err := json.Unmarshal(plugin.Config, &rule); err != nil {
			return nil, fmt.Errorf("decode plugin %q config for ai route %q: %w", plugin.Name, route.Name, err)
		}
	}

	rule[pluginMatchRouteKey] = []string{route.Name}
	if len(route.Domains) > 0 {
		rule[pluginMatchDomainKey] = slices.Clone(route.Domains)
	}
	if route.Model != "" {
		rule[pluginModelKey] = route.Model
	}
	if len(route.Models) > 0 {
		rule[pluginModelsKey] = slices.Clone(route.Models)
	}
	if len(route.PolicyRefs) > 0 {
		rule[pluginPolicyRefsKey] = slices.Clone(route.PolicyRefs)
	}
	if provider, ok := t.aiPluginProviderConfig(route, providers, models); ok {
		// provider 字段是 ai-proxy 插件真正读取的上游模型供应商配置
		// RDS/CDS 决定请求发到哪里，provider 配置决定插件如何改写请求、选择模型和供应商协议
		rule[pluginProviderKey] = provider
	}
	return rule, nil
}

func (t Translator) aiPluginProviderConfig(route AIRoute, providers map[string]AIProvider, models map[string]AIModel) (map[string]any, bool) {
	provider, ok := t.aiRouteProvider(route, providers, models)
	if !ok {
		return nil, false
	}
	config := t.providerConfig(provider)
	if len(route.Models) > 0 {
		model, ok := models[route.Models[0].Name]
		if !ok {
			return nil, false
		}
		if model.ProviderModel != "" {
			config[pluginModelMappingKey] = map[string]string{
				wildcardModel: model.ProviderModel,
			}
		}
		return config, true
	}
	return config, true
}

func (t Translator) aiRouteProvider(route AIRoute, providers map[string]AIProvider, models map[string]AIModel) (AIProvider, bool) {
	if len(route.Models) > 0 {
		model, ok := models[route.Models[0].Name]
		if !ok {
			return AIProvider{}, false
		}
		provider, ok := providers[model.ProviderRef]
		return provider, ok
	}
	if len(route.Providers) == 0 {
		return AIProvider{}, false
	}

	provider, ok := providers[route.Providers[0].Name]
	return provider, ok
}

func (t Translator) aiProviderCluster(provider AIProvider) (Cluster, error) {
	endpoint, err := url.Parse(provider.Endpoint)
	if err != nil {
		return Cluster{}, fmt.Errorf("parse ai provider %q endpoint: %w", provider.Name, err)
	}
	address := endpoint.Hostname()
	port, err := t.endpointPort(endpoint)
	if err != nil {
		return Cluster{}, fmt.Errorf("parse ai provider %q endpoint port: %w", provider.Name, err)
	}

	// AIProvider endpoint 是外部模型服务地址，不走 EDS
	// 这里转成 LOGICAL_DNS cluster，让 Envoy 运行时解析供应商域名
	return Cluster{
		Name:          fmt.Sprintf(aiProviderClusterNameFormat, provider.Name),
		DiscoveryType: ClusterDiscoveryTypeLogicalDNS,
		Address:       address,
		Port:          port,
		TLS:           endpoint.Scheme == httpsScheme,
	}, nil
}

func (t Translator) endpointPort(endpoint *url.URL) (int, error) {
	if endpoint.Port() != "" {
		return strconv.Atoi(endpoint.Port())
	}
	switch endpoint.Scheme {
	case httpScheme:
		return defaultHTTPPort, nil
	case httpsScheme:
		return defaultHTTPSPort, nil
	default:
		return 0, fmt.Errorf("unsupported scheme %q", endpoint.Scheme)
	}
}

func (t Translator) providerConfig(provider AIProvider) map[string]any {
	config := map[string]any{
		pluginProviderNameKey: provider.Name,
		pluginProviderTypeKey: provider.Type,
		pluginProviderURLKey:  provider.Endpoint,
	}
	if provider.Endpoint == "" {
		delete(config, pluginProviderURLKey)
	}
	return config
}

func (t Translator) aiRoutesByName(routes []AIRoute) map[string]AIRoute {
	routesByName := make(map[string]AIRoute, len(routes))
	for _, route := range routes {
		routesByName[route.Name] = route
	}
	return routesByName
}

func (t Translator) aiProvidersByName(providers []AIProvider) map[string]AIProvider {
	providersByName := make(map[string]AIProvider, len(providers))
	for _, provider := range providers {
		providersByName[provider.Name] = provider
	}
	return providersByName
}

func (t Translator) aiModelsByName(models []AIModel) map[string]AIModel {
	modelsByName := make(map[string]AIModel, len(models))
	for _, model := range models {
		modelsByName[model.Name] = model
	}
	return modelsByName
}
