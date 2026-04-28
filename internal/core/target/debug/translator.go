// Package debug 将逻辑网关模型翻译成便于检查的快照
package debug

import (
	"fmt"
	"slices"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/runtime"
)

// Translator 负责生成 debug target 的配置快照
type Translator struct{}

// Config 表示 debug target 的配置载荷
type Config struct {
	Listeners []Listener `json:"listeners"`
	Routes    []Route    `json:"routes"`
	Upstreams []Upstream `json:"upstreams"`
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
	PathPrefix string        `json:"pathPrefix"`
	Methods    []string      `json:"methods"`
	Headers    []HeaderMatch `json:"headers"`
	Upstreams  []UpstreamRef `json:"upstreams"`
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

// Target 返回运行时 target 名称
func (Translator) Target() string {
	return "debug"
}

// Translate 将逻辑网关模型转换成 debug 运行时快照
func (t Translator) Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error) {
	config := Config{
		Listeners: make([]Listener, 0, len(logical.Listeners)),
		Routes:    make([]Route, 0, len(logical.Routes)),
		Upstreams: make([]Upstream, 0, len(logical.Upstreams)),
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
				PathPrefix: rule.PathPrefix,
				Methods:    slices.Clone(rule.Methods),
				Upstreams:  make([]UpstreamRef, 0, len(rule.Upstreams)),
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

	return runtime.RuntimeSnapshot{
		Target:  t.Target(),
		Gateway: logical.Name,
		Version: fmt.Sprintf("debug/%s", logical.Name),
		Config:  config,
	}, nil
}
