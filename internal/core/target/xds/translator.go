// Package xds 将逻辑网关模型翻译成 Envoy-oriented 配置
package xds

import (
	"fmt"
	"slices"

	"github.com/lgc202/ingate/internal/core/ir"
	"github.com/lgc202/ingate/internal/core/runtime"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	pluginacl "github.com/lgc202/ingate/pkg/plugin/acl"
	pluginratelimit "github.com/lgc202/ingate/pkg/plugin/ratelimit"
)

const (
	targetName     string = "xds"
	versionPrefix  string = "xds/%s"
	wildcardDomain string = "*"
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
	RateLimit           *RateLimitConfig     `json:"rateLimit,omitempty"`
	AccessControl       *AccessControlConfig `json:"accessControl,omitempty"`
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
	GatewayName             string            `json:"gatewayName"`
	Name                    string            `json:"name"`
	RuleName                string            `json:"ruleName,omitempty"`
	Match                   RouteMatch        `json:"match"`
	TimeoutMillis           int               `json:"timeoutMillis"`
	RequestHeadersToAdd     []HeaderValue     `json:"requestHeadersToAdd,omitempty"`
	RequestHeadersToRemove  []string          `json:"requestHeadersToRemove,omitempty"`
	ResponseHeadersToAdd    []HeaderValue     `json:"responseHeadersToAdd,omitempty"`
	ResponseHeadersToRemove []string          `json:"responseHeadersToRemove,omitempty"`
	RetryPolicy             *RetryPolicy      `json:"retryPolicy,omitempty"`
	WeightedClusters        []WeightedCluster `json:"weightedClusters"`
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
				if len(rule.ResponseHeadersToAdd) > 0 {
					xdsRoute.ResponseHeadersToAdd = make([]HeaderValue, 0, len(rule.ResponseHeadersToAdd))
					for _, header := range rule.ResponseHeadersToAdd {
						xdsRoute.ResponseHeadersToAdd = append(xdsRoute.ResponseHeadersToAdd, HeaderValue{
							Name:  header.Name,
							Value: header.Value,
						})
					}
				}
				xdsRoute.ResponseHeadersToRemove = slices.Clone(rule.ResponseHeadersToRemove)
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

	config.RateLimit = t.translateRateLimitConfig(logical)
	config.AccessControl = t.translateAccessControlConfig(logical)

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf(versionPrefix, logical.Name),
		Config:  config,
	}, nil
}

func (t Translator) translateAccessControlConfig(logical ir.LogicalGateway) *AccessControlConfig {
	if len(logical.AccessControlPolicies) == 0 || len(logical.PolicyBindings) == 0 {
		return nil
	}

	policiesByName := make(map[string]ir.LogicalAccessControlPolicy, len(logical.AccessControlPolicies))
	for _, policy := range logical.AccessControlPolicies {
		policiesByName[policy.Name] = policy
	}

	config := &AccessControlConfig{
		Bindings: make([]pluginacl.Binding, 0, len(logical.PolicyBindings)),
	}
	for _, binding := range logical.PolicyBindings {
		accessControlBinding := pluginacl.Binding{
			Name: binding.Name,
			Target: pluginacl.Target{
				Kind:     string(binding.Target.Kind),
				Name:     binding.Target.Name,
				RuleName: binding.Target.RuleName,
			},
			Policies: make([]pluginacl.Policy, 0, len(binding.Policies)),
		}
		for _, policyRef := range binding.Policies {
			if policyRef.Kind != resource.KindAccessControlPolicy {
				continue
			}
			policy, ok := policiesByName[policyRef.Name]
			if !ok {
				continue
			}
			accessControlBinding.Policies = append(accessControlBinding.Policies, t.accessControlPolicy(policy))
		}
		if len(accessControlBinding.Policies) == 0 {
			continue
		}
		config.Bindings = append(config.Bindings, accessControlBinding)
	}
	if len(config.Bindings) == 0 {
		return nil
	}
	return config
}

func (t Translator) accessControlPolicy(policy ir.LogicalAccessControlPolicy) pluginacl.Policy {
	result := pluginacl.Policy{
		Name:          policy.Name,
		DisplayName:   policy.DisplayName,
		DefaultAction: pluginacl.Action(policy.DefaultAction),
		Rules:         make([]pluginacl.Rule, 0, len(policy.Rules)),
		Response: pluginacl.Response{
			StatusCode: policy.Response.StatusCode,
			Message:    policy.Response.Message,
		},
	}
	for _, rule := range policy.Rules {
		pluginRule := pluginacl.Rule{
			Name:       rule.Name,
			Action:     pluginacl.Action(rule.Action),
			Conditions: make([]pluginacl.Condition, 0, len(rule.Conditions)),
		}
		for _, condition := range rule.Conditions {
			pluginRule.Conditions = append(pluginRule.Conditions, pluginacl.Condition{
				Type:  pluginacl.ConditionType(condition.Type),
				Name:  condition.Name,
				Value: condition.Value,
			})
		}
		result.Rules = append(result.Rules, pluginRule)
	}
	return result
}

func (t Translator) translateRateLimitConfig(logical ir.LogicalGateway) *RateLimitConfig {
	if len(logical.RateLimitPolicies) == 0 || len(logical.PolicyBindings) == 0 {
		return nil
	}

	policiesByName := make(map[string]ir.LogicalRateLimitPolicy, len(logical.RateLimitPolicies))
	for _, policy := range logical.RateLimitPolicies {
		policiesByName[policy.Name] = policy
	}

	config := &RateLimitConfig{
		Bindings: make([]pluginratelimit.Binding, 0, len(logical.PolicyBindings)),
	}
	for _, binding := range logical.PolicyBindings {
		rateLimitBinding := pluginratelimit.Binding{
			Name: binding.Name,
			Target: pluginratelimit.Target{
				Kind:     string(binding.Target.Kind),
				Name:     binding.Target.Name,
				RuleName: binding.Target.RuleName,
			},
			Policies: make([]pluginratelimit.Policy, 0, len(binding.Policies)),
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
	if len(config.Bindings) == 0 {
		return nil
	}
	return config
}

func (t Translator) rateLimitPolicy(policy ir.LogicalRateLimitPolicy) pluginratelimit.Policy {
	result := pluginratelimit.Policy{
		Name:          policy.Name,
		Mode:          pluginratelimit.Mode(policy.Mode),
		Rules:         make([]pluginratelimit.Rule, 0, len(policy.Rules)),
		Response:      pluginratelimit.Response(policy.Response),
		FailurePolicy: pluginratelimit.FailurePolicy(policy.FailurePolicy),
	}
	for _, rule := range policy.Rules {
		result.Rules = append(result.Rules, pluginratelimit.Rule{
			Name:      rule.Name,
			Key:       t.rateLimitKey(rule.Key),
			Limit:     pluginratelimit.Quota(rule.Limit),
			Algorithm: pluginratelimit.Algorithm(rule.Algorithm),
		})
	}
	return result
}

func (t Translator) rateLimitKey(parts []ir.LogicalRateLimitKeyPart) []pluginratelimit.KeyPart {
	result := make([]pluginratelimit.KeyPart, 0, len(parts))
	for _, part := range parts {
		result = append(result, pluginratelimit.KeyPart{
			Type: pluginratelimit.KeyType(part.Type),
			Name: part.Name,
		})
	}
	return result
}
