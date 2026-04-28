// Package debug translates logical gateway intent into inspectable snapshots.
package debug

import (
	"fmt"

	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/runtime"
)

// Translator translates logical gateway intent for the debug target.
type Translator struct{}

// Config is the debug target configuration payload.
type Config struct {
	Listeners []Listener
	Routes    []Route
	Upstreams []Upstream
}

// Listener is a debug listener payload.
type Listener struct {
	Name     string
	Protocol string
	Port     int
	Hostname string
}

// Route is a debug route payload.
type Route struct {
	Name      string
	Hostnames []string
	Rules     []RouteRule
}

// RouteRule is a debug route rule payload.
type RouteRule struct {
	PathPrefix string
	Upstreams  []UpstreamRef
}

// UpstreamRef is a debug upstream reference payload.
type UpstreamRef struct {
	Name   string
	Weight int
}

// Upstream is a debug upstream payload.
type Upstream struct {
	Name      string
	Endpoints []Endpoint
}

// Endpoint is a debug endpoint payload.
type Endpoint struct {
	Address string
	Port    int
}

// Translate converts logical gateway intent into a debug runtime snapshot.
func (Translator) Translate(logical ir.LogicalGateway) runtime.RuntimeSnapshot {
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
			Hostnames: append([]string(nil), route.Hostnames...),
			Rules:     make([]RouteRule, 0, len(route.Rules)),
		}
		for _, rule := range route.Rules {
			debugRule := RouteRule{
				PathPrefix: rule.PathPrefix,
				Upstreams:  make([]UpstreamRef, 0, len(rule.Upstreams)),
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
		Target:  "debug",
		Gateway: logical.Name,
		Version: fmt.Sprintf("debug/%s", logical.Name),
		Config:  config,
	}
}
