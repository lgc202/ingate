// Package ir defines runtime-neutral compiled gateway intent.
package ir

// LogicalGateway is the runtime-neutral result of compiling one gateway.
type LogicalGateway struct {
	Name      string
	Listeners []LogicalListener
	Routes    []LogicalRoute
	Upstreams []LogicalUpstream
}

// LogicalListener is a compiled gateway listener.
type LogicalListener struct {
	Name     string
	Protocol string
	Port     int
	Hostname string
}

// LogicalRoute is a compiled route attached to a gateway.
type LogicalRoute struct {
	Name      string
	Hostnames []string
	Rules     []LogicalRouteRule
}

// LogicalRouteRule is a compiled route rule.
type LogicalRouteRule struct {
	PathPrefix string
	Upstreams  []LogicalUpstreamRef
}

// LogicalUpstreamRef is a resolved upstream reference.
type LogicalUpstreamRef struct {
	Name   string
	Weight int
}

// LogicalUpstream is a compiled upstream used by attached routes.
type LogicalUpstream struct {
	Name      string
	Endpoints []LogicalEndpoint
}

// LogicalEndpoint is a compiled upstream endpoint.
type LogicalEndpoint struct {
	Address string
	Port    int
}
