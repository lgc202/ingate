package translate

import (
	"fmt"
	"strings"

	compilerir "github.com/lgc202/ingate/internal/controlplane/compiler/ir"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type RuntimeConfig struct {
	Key                   shared.ObjectKey
	GatewayName           string
	Version               string
	ObservedGeneration    int64
	GatewayAuthSummary    *RuntimeAuthSummary
	GatewayTrafficSummary *RuntimeTrafficSummary
	Listeners             []RuntimeListener
	Routes                []RuntimeRoute
	Backends              []RuntimeBackend
}

type RuntimeListener struct {
	Name      string
	Protocol  string
	Port      uint32
	Hostnames []string
	TLS       *RuntimeListenerTLS
}

type RuntimeListenerTLS struct {
	Mode            string
	CertificateName string
	SecretName      string
	Domains         []string
}

type RuntimeRoute struct {
	Name           string
	Hostnames      []string
	Rules          []RuntimeRouteRule
	AuthSummary    *RuntimeAuthSummary
	TrafficSummary *RuntimeTrafficSummary
}

type RuntimeRouteRule struct {
	Matches         []RuntimeRouteMatch
	BackendRefs     []RuntimeBackendRef
	URLRewrite      *RuntimeURLRewrite
	RequestHeaders  *RuntimeHeaderOperations
	ResponseHeaders *RuntimeHeaderOperations
}

type RuntimeRouteMatch struct {
	Path    *RuntimePathMatch
	Method  string
	Headers []RuntimeHeaderMatch
}

type RuntimePathMatch struct {
	Type  string
	Value string
}

type RuntimeHeaderMatch struct {
	Name  string
	Value string
}

type RuntimeBackendRef struct {
	Name   string
	Port   uint32
	Weight uint32
}

type RuntimeURLRewrite struct {
	ReplacePrefixMatch string
}

type RuntimeHeaderOperations struct {
	Set    []RuntimeHeader
	Add    []RuntimeHeader
	Remove []string
}

type RuntimeHeader struct {
	Name  string
	Value string
}

type RuntimeBackend struct {
	Name           string
	Type           string
	Protocol       string
	DefaultPort    uint32
	LoadBalancing  string
	Endpoints      []RuntimeEndpoint
	AuthSummary    *RuntimeAuthSummary
	TrafficSummary *RuntimeTrafficSummary
}

type RuntimeEndpoint struct {
	Address string
	Port    uint32
	Weight  uint32
	Healthy bool
}

type RuntimeAuthSummary struct {
	Policies []RuntimePolicyRef
}

type RuntimePolicyRef struct {
	Kind string
	Name string
	Type string
}

type RuntimeTrafficSummary struct {
	Policies []RuntimeTrafficPolicyRef
}

type RuntimeTrafficPolicyRef struct {
	Kind              string
	Name              string
	TimeoutDuration   string
	RetryAttempts     int32
	RetryConditions   []string
	RateLimitRequests int32
	RateLimitUnit     string
	RateLimitScope    string
}

func FromLogicalGateway(gateway *compilerir.LogicalGateway) (*RuntimeConfig, error) {
	if gateway == nil {
		return nil, fmt.Errorf("logical gateway must not be nil")
	}
	if strings.TrimSpace(gateway.Meta.Name) == "" {
		return nil, fmt.Errorf("logical gateway name must not be empty")
	}

	version := strings.TrimSpace(gateway.Meta.Version)
	if version == "" {
		version = fmt.Sprintf("logical-%s", gateway.Meta.Name)
	}

	out := &RuntimeConfig{
		Key:                   shared.NewObjectKey(gateway.Meta.Namespace, gateway.Meta.Name),
		GatewayName:           gateway.Meta.Name,
		Version:               version,
		GatewayAuthSummary:    authSummaryFromLogical(gateway.Policies.GatewayAuth),
		GatewayTrafficSummary: trafficSummaryFromLogical(gateway.Policies.GatewayTraffic),
		Listeners:             make([]RuntimeListener, 0, len(gateway.Listeners)),
		Routes:                make([]RuntimeRoute, 0, len(gateway.Routes)),
		Backends:              make([]RuntimeBackend, 0, len(gateway.Backends)),
	}

	for _, listener := range gateway.Listeners {
		out.Listeners = append(out.Listeners, listenerFromLogical(listener))
	}
	for _, route := range gateway.Routes {
		out.Routes = append(out.Routes, routeFromLogical(route))
	}
	for _, backend := range gateway.Backends {
		out.Backends = append(out.Backends, backendFromLogical(backend))
	}

	return out, nil
}

func listenerFromLogical(listener compilerir.Listener) RuntimeListener {
	out := RuntimeListener{
		Name:      listener.Name,
		Protocol:  listener.Protocol,
		Port:      toUint32(listener.Port),
		Hostnames: cloneStrings(listener.Hostnames),
	}
	if listener.TLS != nil {
		out.TLS = &RuntimeListenerTLS{
			Mode:            listener.TLS.Mode,
			CertificateName: listener.TLS.CertificateName,
			SecretName:      listener.TLS.SecretName,
			Domains:         cloneStrings(listener.TLS.Domains),
		}
	}
	return out
}

func routeFromLogical(route compilerir.Route) RuntimeRoute {
	out := RuntimeRoute{
		Name:           route.Name,
		Hostnames:      cloneStrings(route.Hostnames),
		Rules:          make([]RuntimeRouteRule, 0, len(route.Rules)),
		AuthSummary:    authSummaryFromLogical(route.AuthSummary),
		TrafficSummary: trafficSummaryFromLogical(route.TrafficSummary),
	}
	for _, rule := range route.Rules {
		out.Rules = append(out.Rules, routeRuleFromLogical(rule))
	}
	return out
}

func routeRuleFromLogical(rule compilerir.RouteRule) RuntimeRouteRule {
	out := RuntimeRouteRule{
		Matches:     make([]RuntimeRouteMatch, 0, len(rule.Matches)),
		BackendRefs: make([]RuntimeBackendRef, 0, len(rule.BackendRefs)),
	}
	for _, match := range rule.Matches {
		out.Matches = append(out.Matches, routeMatchFromResolved(match))
	}
	for _, backendRef := range rule.BackendRefs {
		out.BackendRefs = append(out.BackendRefs, RuntimeBackendRef{
			Name:   backendRef.Name,
			Port:   toUint32(backendRef.Port),
			Weight: toUint32(backendRef.Weight),
		})
	}
	for _, filter := range rule.Filters {
		switch filter.Type {
		case "URLRewrite":
			if filter.URLRewrite != nil && filter.URLRewrite.Path != nil {
				out.URLRewrite = &RuntimeURLRewrite{ReplacePrefixMatch: filter.URLRewrite.Path.ReplacePrefixMatch}
			}
		case "RequestHeaderModifier":
			out.RequestHeaders = headerOpsFromResolved(filter.RequestHeaderModifier)
		case "ResponseHeaderModifier":
			out.ResponseHeaders = headerOpsFromResolved(filter.ResponseHeaderModifier)
		}
	}
	return out
}

func routeMatchFromResolved(match gatewayv1alpha1.HTTPRouteMatch) RuntimeRouteMatch {
	out := RuntimeRouteMatch{
		Method:  match.Method,
		Headers: make([]RuntimeHeaderMatch, 0, len(match.Headers)),
	}
	if match.Path != nil {
		out.Path = &RuntimePathMatch{Type: match.Path.Type, Value: match.Path.Value}
	}
	for _, header := range match.Headers {
		out.Headers = append(out.Headers, RuntimeHeaderMatch{Name: header.Name, Value: header.Value})
	}
	return out
}

func headerOpsFromResolved(filter *gatewayv1alpha1.HTTPHeaderFilter) *RuntimeHeaderOperations {
	if filter == nil {
		return nil
	}
	out := &RuntimeHeaderOperations{
		Set:    make([]RuntimeHeader, 0, len(filter.Set)),
		Add:    make([]RuntimeHeader, 0, len(filter.Add)),
		Remove: cloneStrings(filter.Remove),
	}
	for _, header := range filter.Set {
		out.Set = append(out.Set, RuntimeHeader{Name: header.Name, Value: header.Value})
	}
	for _, header := range filter.Add {
		out.Add = append(out.Add, RuntimeHeader{Name: header.Name, Value: header.Value})
	}
	return out
}

func backendFromLogical(backend compilerir.Backend) RuntimeBackend {
	out := RuntimeBackend{
		Name:           backend.Name,
		Protocol:       backend.Protocol,
		DefaultPort:    toUint32(backend.DefaultPort),
		Endpoints:      make([]RuntimeEndpoint, 0, len(backend.Endpoints)),
		AuthSummary:    authSummaryFromLogical(backend.AuthSummary),
		TrafficSummary: trafficSummaryFromLogical(backend.TrafficSummary),
	}
	if backend.LoadBalance != nil {
		out.LoadBalancing = backend.LoadBalance.Policy
	}
	for _, endpoint := range backend.Endpoints {
		out.Endpoints = append(out.Endpoints, RuntimeEndpoint{
			Address: endpoint.Address,
			Port:    toUint32(endpoint.Port),
			Weight:  toUint32(endpoint.Weight),
			Healthy: endpoint.Healthy,
		})
	}
	return out
}

func authSummaryFromLogical(summary *compilerir.AuthSummary) *RuntimeAuthSummary {
	if summary == nil {
		return nil
	}
	out := &RuntimeAuthSummary{Policies: make([]RuntimePolicyRef, 0, len(summary.Policies))}
	for _, policy := range summary.Policies {
		out.Policies = append(out.Policies, RuntimePolicyRef{
			Kind: policy.Kind,
			Name: policy.Name,
			Type: policy.Type,
		})
	}
	return out
}

func trafficSummaryFromLogical(summary *compilerir.TrafficSummary) *RuntimeTrafficSummary {
	if summary == nil {
		return nil
	}
	out := &RuntimeTrafficSummary{Policies: make([]RuntimeTrafficPolicyRef, 0, len(summary.Policies))}
	for _, policy := range summary.Policies {
		out.Policies = append(out.Policies, RuntimeTrafficPolicyRef{
			Kind:              policy.Kind,
			Name:              policy.Name,
			TimeoutDuration:   policy.TimeoutDuration,
			RetryAttempts:     policy.RetryAttempts,
			RetryConditions:   cloneStrings(policy.RetryConditions),
			RateLimitRequests: policy.RateLimitRequests,
			RateLimitUnit:     policy.RateLimitUnit,
			RateLimitScope:    policy.RateLimitScope,
		})
	}
	return out
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func toUint32(value int32) uint32 {
	if value <= 0 {
		return 0
	}
	return uint32(value)
}
