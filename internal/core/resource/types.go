// Package resource defines declared control-plane resources.
package resource

// Bundle groups resources for one in-memory compilation run.
type Bundle struct {
	Gateways  []Gateway
	Routes    []Route
	Upstreams []Upstream
}

// Metadata identifies a declared resource.
type Metadata struct {
	Name string
}

// Gateway declares a traffic entry point.
type Gateway struct {
	Metadata Metadata
	Spec     GatewaySpec
}

// GatewaySpec defines gateway listeners.
type GatewaySpec struct {
	Listeners []Listener
}

// Listener declares one gateway listening socket.
type Listener struct {
	Name     string
	Protocol string
	Port     int
	Hostname string
}

// Route declares request matches and upstream references.
type Route struct {
	Metadata Metadata
	Spec     RouteSpec
}

// RouteSpec defines how a route attaches to gateways.
type RouteSpec struct {
	ParentRefs []string
	Hostnames  []string
	Rules      []RouteRule
}

// RouteRule declares one route match and weighted upstream set.
type RouteRule struct {
	PathPrefix   string
	UpstreamRefs []UpstreamRef
}

// UpstreamRef references an upstream from a route rule.
type UpstreamRef struct {
	Name   string
	Weight int
}

// Upstream declares a logical upstream service.
type Upstream struct {
	Metadata Metadata
	Spec     UpstreamSpec
}

// UpstreamSpec defines upstream endpoints.
type UpstreamSpec struct {
	Endpoints []Endpoint
}

// Endpoint declares one upstream endpoint.
type Endpoint struct {
	Address string
	Port    int
}
