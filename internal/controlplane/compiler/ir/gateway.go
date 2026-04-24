package ir

import gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"

type LogicalGateway struct {
	Meta      GatewayMeta
	Listeners []Listener
	Routes    []Route
	Backends  []Backend
	Policies  Policies
	Trace     *TraceInfo
}

type GatewayMeta struct {
	Namespace string
	Name      string
	Version   string
}

type Listener struct {
	Name      string
	Protocol  string
	Port      int32
	Hostnames []string
	TLS       *ListenerTLS
}

type ListenerTLS struct {
	Mode            string
	CertificateName string
	SecretName      string
	Domains         []string
}

type Route struct {
	Name           string
	Hostnames      []string
	Rules          []RouteRule
	AuthSummary    *AuthSummary
	TrafficSummary *TrafficSummary
}

type RouteRule struct {
	Matches     []gatewayv1alpha1.HTTPRouteMatch
	BackendRefs []gatewayv1alpha1.BackendRef
	Filters     []gatewayv1alpha1.HTTPRouteFilter
}

type Backend struct {
	Name           string
	Protocol       string
	DefaultPort    int32
	LoadBalance    *gatewayv1alpha1.LoadBalanceSpec
	Endpoints      []gatewayv1alpha1.BackendEndpoint
	AuthSummary    *AuthSummary
	TrafficSummary *TrafficSummary
}

type Policies struct {
	GatewayAuth    *AuthSummary
	GatewayTraffic *TrafficSummary
}

type AuthSummary struct {
	Policies []PolicyRef
}

type PolicyRef struct {
	Kind string
	Name string
	Type string
}

type TrafficSummary struct {
	Policies []TrafficPolicyRef
}

type TrafficPolicyRef struct {
	Kind string
	Name string

	TimeoutDuration string
	RetryAttempts   int32
	RetryConditions []string

	RateLimitRequests int32
	RateLimitUnit     string
	RateLimitScope    string
}

type TraceInfo struct {
	Sources []SourceRef
}

type SourceRef struct {
	Kind      string
	Namespace string
	Name      string
}
