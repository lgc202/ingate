package config

import (
	"cmp"
	"fmt"
	"slices"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

type compileContext struct {
	resources ResourceSet

	gateways              map[string]*gatewayv1.Gateway
	routes                map[string]*gatewayv1.Route
	upstreams             map[string]*gatewayv1.Upstream
	rateLimitPolicies     map[string]*gatewayv1.RateLimitPolicy
	accessControlPolicies map[string]*gatewayv1.AccessControlPolicy
	policyBindings        map[string]*gatewayv1.PolicyBinding
	routeRules            map[string]map[string]bool

	listenerGroups   map[listenerKey]*listenerGroup
	gatewayListeners map[string][]gatewayListener
	routeAttachments []routeAttachment
	diagnostics      []Diagnostic
	diagnosticSet    map[string]bool
}

// Compile 将完整资源集合直接编译为可发布的 Envoy 配置
func (Compiler) Compile(resources ResourceSet) CompileResult {
	c := &compileContext{
		resources:             resources,
		gateways:              make(map[string]*gatewayv1.Gateway, len(resources.Gateways)),
		routes:                make(map[string]*gatewayv1.Route, len(resources.Routes)),
		upstreams:             make(map[string]*gatewayv1.Upstream, len(resources.Upstreams)),
		rateLimitPolicies:     make(map[string]*gatewayv1.RateLimitPolicy, len(resources.RateLimitPolicies)),
		accessControlPolicies: make(map[string]*gatewayv1.AccessControlPolicy, len(resources.AccessControlPolicies)),
		policyBindings:        make(map[string]*gatewayv1.PolicyBinding, len(resources.PolicyBindings)),
		routeRules:            make(map[string]map[string]bool, len(resources.Routes)),
		listenerGroups:        make(map[listenerKey]*listenerGroup),
		gatewayListeners:      make(map[string][]gatewayListener),
		diagnosticSet:         make(map[string]bool),
	}

	c.indexResources()
	clusters, endpoints := c.buildUpstreams()
	c.buildListenerGroups()
	routes := c.buildRoutes()
	policies := c.buildPolicyConfigs()
	listeners := c.buildListeners(policies)

	config := Config{
		Listeners: listeners,
		Routes:    routes,
		Clusters:  clusters,
		Endpoints: endpoints,
	}
	sortConfig(&config)
	c.sortDiagnostics()

	result := CompileResult{Diagnostics: c.diagnostics}
	if result.HasErrors() {
		return result
	}

	version, err := config.version()
	if err != nil {
		c.addDiagnostic(SeverityError, "", "envoy", ReasonCompileFailed, fmt.Sprintf("compute Envoy config version: %v", err))
		c.sortDiagnostics()
		return CompileResult{Diagnostics: c.diagnostics}
	}
	if _, err := config.Snapshot(version); err != nil {
		c.addDiagnostic(SeverityError, "", "envoy", ReasonCompileFailed, fmt.Sprintf("build consistent Envoy snapshot: %v", err))
		c.sortDiagnostics()
		return CompileResult{Diagnostics: c.diagnostics}
	}

	result.Version = version
	result.Config = config
	return result
}

func (c *compileContext) indexResources() {
	for _, gateway := range c.resources.Gateways {
		if gateway == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, "", ReasonInvalidSpec, "gateway resource is nil")
			continue
		}
		c.indexGateway(gateway)
	}
	for _, route := range c.resources.Routes {
		if route == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, "", ReasonInvalidSpec, "route resource is nil")
			continue
		}
		c.indexRoute(route)
	}
	for _, upstream := range c.resources.Upstreams {
		if upstream == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, "", ReasonInvalidSpec, "upstream resource is nil")
			continue
		}
		c.indexUpstream(upstream)
	}
	for _, policy := range c.resources.RateLimitPolicies {
		if policy == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, "", ReasonInvalidSpec, "rate limit policy resource is nil")
			continue
		}
		c.indexRateLimitPolicy(policy)
	}
	for _, policy := range c.resources.AccessControlPolicies {
		if policy == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, "", ReasonInvalidSpec, "access control policy resource is nil")
			continue
		}
		c.indexAccessControlPolicy(policy)
	}
	for _, binding := range c.resources.PolicyBindings {
		if binding == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindPolicyBinding, "", ReasonInvalidSpec, "policy binding resource is nil")
			continue
		}
		c.indexPolicyBinding(binding)
	}
}

func (c *compileContext) indexGateway(gateway *gatewayv1.Gateway) {
	id := gateway.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, id, ReasonInvalidSpec, "gateway metadata.name is required")
		return
	}
	if _, ok := c.gateways[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindGateway, id, ReasonConflict, fmt.Sprintf("duplicate gateway %q", id))
		return
	}
	c.gateways[id] = gateway
}

func (c *compileContext) indexRoute(route *gatewayv1.Route) {
	id := route.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, id, ReasonInvalidSpec, "route metadata.name is required")
		return
	}
	if _, ok := c.routes[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, id, ReasonConflict, fmt.Sprintf("duplicate route %q", id))
		return
	}
	c.routes[id] = route

	rules := make(map[string]bool, len(route.Spec.Rules))
	for _, rule := range route.Spec.Rules {
		if rule.Name == "" {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, id, ReasonInvalidSpec, fmt.Sprintf("route %q has a rule without a name", id))
			continue
		}
		if rules[rule.Name] {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, id, ReasonConflict, fmt.Sprintf("route %q has duplicate rule %q", id, rule.Name))
			continue
		}
		rules[rule.Name] = true
	}
	c.routeRules[id] = rules
}

func (c *compileContext) indexUpstream(upstream *gatewayv1.Upstream) {
	id := upstream.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, id, ReasonInvalidSpec, "upstream metadata.name is required")
		return
	}
	if _, ok := c.upstreams[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, id, ReasonConflict, fmt.Sprintf("duplicate upstream %q", id))
		return
	}
	c.upstreams[id] = upstream
}

func (c *compileContext) indexRateLimitPolicy(policy *gatewayv1.RateLimitPolicy) {
	id := policy.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, id, ReasonInvalidSpec, "rate limit policy metadata.name is required")
		return
	}
	if _, ok := c.rateLimitPolicies[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, id, ReasonConflict, fmt.Sprintf("duplicate rate limit policy %q", id))
		return
	}
	c.rateLimitPolicies[id] = policy
}

func (c *compileContext) indexAccessControlPolicy(policy *gatewayv1.AccessControlPolicy) {
	id := policy.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, id, ReasonInvalidSpec, "access control policy metadata.name is required")
		return
	}
	if _, ok := c.accessControlPolicies[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindAccessControlPolicy, id, ReasonConflict, fmt.Sprintf("duplicate access control policy %q", id))
		return
	}
	c.accessControlPolicies[id] = policy
}

func (c *compileContext) indexPolicyBinding(binding *gatewayv1.PolicyBinding) {
	id := binding.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindPolicyBinding, id, ReasonInvalidSpec, "policy binding metadata.name is required")
		return
	}
	if _, ok := c.policyBindings[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindPolicyBinding, id, ReasonConflict, fmt.Sprintf("duplicate policy binding %q", id))
		return
	}
	c.policyBindings[id] = binding
}

func (c *compileContext) addDiagnostic(severity Severity, kind gatewayv1.Kind, id string, reason Reason, message string) {
	key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", severity, kind, id, reason, message)
	if c.diagnosticSet[key] {
		return
	}
	c.diagnosticSet[key] = true
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Severity: severity,
		Kind:     kind,
		ID:       id,
		Reason:   reason,
		Message:  message,
	})
}

func (c *compileContext) sortDiagnostics() {
	slices.SortFunc(c.diagnostics, func(a, b Diagnostic) int {
		if a.Severity != b.Severity {
			if a.Severity == SeverityError {
				return -1
			}
			if b.Severity == SeverityError {
				return 1
			}
			return cmp.Compare(a.Severity, b.Severity)
		}
		if a.Kind != b.Kind {
			return cmp.Compare(a.Kind, b.Kind)
		}
		if a.ID != b.ID {
			return cmp.Compare(a.ID, b.ID)
		}
		if a.Reason != b.Reason {
			return cmp.Compare(a.Reason, b.Reason)
		}
		return cmp.Compare(a.Message, b.Message)
	})
}

func sortConfig(config *Config) {
	slices.SortFunc(config.Listeners, func(a, b *listenerv3.Listener) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	slices.SortFunc(config.Routes, func(a, b *routev3.RouteConfiguration) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	slices.SortFunc(config.Clusters, func(a, b *clusterv3.Cluster) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})
	slices.SortFunc(config.Endpoints, func(a, b *endpointv3.ClusterLoadAssignment) int {
		return cmp.Compare(a.GetClusterName(), b.GetClusterName())
	})
}
