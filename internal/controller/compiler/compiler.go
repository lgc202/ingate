package compiler

import (
	"cmp"
	"fmt"
	"slices"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

type compilation struct {
	resources Resources

	gateways              map[string]*gatewayv1.Gateway
	certificates          map[string]*gatewayv1.Certificate
	validCertificates     map[string]bool
	routes                map[string]*gatewayv1.Route
	upstreams             map[string]*gatewayv1.Upstream
	upstreamClusters      map[string]string
	rateLimitPolicies     map[string]*gatewayv1.RateLimitPolicy
	accessControlPolicies map[string]*gatewayv1.AccessControlPolicy
	tokenQuotaPolicies    map[string]*gatewayv1.TokenQuotaPolicy

	listenerGroups   map[listenerKey]*listenerGroup
	gatewayListeners map[string][]gatewayListener
	routeAttachments []routeAttachment
	aiRoutes         map[aiRouteKey]compiledAIRoute
	policyTargetSet  map[CompiledPolicyTarget]bool
	diagnostics      []Diagnostic
	diagnosticSet    map[string]bool
}

// Compile 将完整资源集合直接编译为可发布的 Envoy 配置
func Compile(resources Resources) Result {
	c := &compilation{
		resources:             resources,
		gateways:              make(map[string]*gatewayv1.Gateway, len(resources.Gateways)),
		certificates:          make(map[string]*gatewayv1.Certificate, len(resources.Certificates)),
		validCertificates:     make(map[string]bool, len(resources.Certificates)),
		routes:                make(map[string]*gatewayv1.Route, len(resources.Routes)),
		upstreams:             make(map[string]*gatewayv1.Upstream, len(resources.Upstreams)),
		upstreamClusters:      make(map[string]string, len(resources.Upstreams)),
		rateLimitPolicies:     make(map[string]*gatewayv1.RateLimitPolicy, len(resources.RateLimitPolicies)),
		accessControlPolicies: make(map[string]*gatewayv1.AccessControlPolicy, len(resources.AccessControlPolicies)),
		tokenQuotaPolicies:    make(map[string]*gatewayv1.TokenQuotaPolicy, len(resources.TokenQuotaPolicies)),
		listenerGroups:        make(map[listenerKey]*listenerGroup),
		gatewayListeners:      make(map[string][]gatewayListener),
		aiRoutes:              make(map[aiRouteKey]compiledAIRoute),
		policyTargetSet:       make(map[CompiledPolicyTarget]bool),
		diagnosticSet:         make(map[string]bool),
	}

	c.indexResources()
	clusters, endpoints := c.buildUpstreams()
	c.buildListenerGroups()
	routes := c.buildRoutes()
	plugins := c.buildPolicyConfigs()
	c.addAIProxyConfigs(plugins)
	listeners := c.buildListeners(plugins)

	envoyConfig := EnvoyConfig{
		Listeners: listeners,
		Routes:    routes,
		Clusters:  clusters,
		Endpoints: endpoints,
	}
	sortEnvoyConfig(&envoyConfig)
	c.sortDiagnostics()

	resourceGenerations := resources.Generations()
	result := Result{
		ResourceGenerations: resourceGenerations,
		Diagnostics:         c.diagnostics,
	}
	if result.HasErrors() {
		return result
	}

	version, err := envoyConfig.version()
	if err != nil {
		c.addDiagnostic(SeverityError, "", "envoy", ReasonCompileFailed, fmt.Sprintf("compute Envoy config version: %v", err))
		c.sortDiagnostics()
		return Result{ResourceGenerations: resourceGenerations, Diagnostics: c.diagnostics}
	}
	if _, err := envoyConfig.Snapshot(version); err != nil {
		c.addDiagnostic(SeverityError, "", "envoy", ReasonCompileFailed, fmt.Sprintf("build consistent Envoy snapshot: %v", err))
		c.sortDiagnostics()
		return Result{ResourceGenerations: resourceGenerations, Diagnostics: c.diagnostics}
	}

	result.Version = version
	result.Config = envoyConfig
	result.PolicyTargets = c.compiledPolicyTargets()
	return result
}

func (c *compilation) indexResources() {
	for _, certificate := range c.resources.Certificates {
		if certificate == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindCertificate, "", ReasonInvalidSpec, "certificate resource is nil")
			continue
		}
		c.indexCertificate(certificate)
	}
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
	for _, policy := range c.resources.TokenQuotaPolicies {
		if policy == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindTokenQuotaPolicy, "", ReasonInvalidSpec, "token quota policy resource is nil")
			continue
		}
		c.indexTokenQuotaPolicy(policy)
	}
}

func (c *compilation) indexCertificate(certificate *gatewayv1.Certificate) {
	id := certificate.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindCertificate, id, ReasonInvalidSpec, "certificate metadata.name is required")
		return
	}
	if _, ok := c.certificates[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindCertificate, id, ReasonConflict, fmt.Sprintf("duplicate certificate %q", id))
		return
	}
	c.certificates[id] = certificate

	if certificate.Spec.CertificatePEM == "" {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q certificatePEM is required", id),
		)
		return
	}
	if certificate.Spec.PrivateKeyPEM == "" {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q privateKeyPEM is required", id),
		)
		return
	}
	if _, err := certificateutil.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM); err != nil {
		c.addDiagnostic(
			SeverityError,
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q contains an invalid TLS key pair: %v", id, err),
		)
		return
	}
	c.validCertificates[id] = true
}

func (c *compilation) indexGateway(gateway *gatewayv1.Gateway) {
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

func (c *compilation) indexRoute(route *gatewayv1.Route) {
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
}

func (c *compilation) indexUpstream(upstream *gatewayv1.Upstream) {
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

func (c *compilation) indexRateLimitPolicy(policy *gatewayv1.RateLimitPolicy) {
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

func (c *compilation) indexAccessControlPolicy(policy *gatewayv1.AccessControlPolicy) {
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

func (c *compilation) indexTokenQuotaPolicy(policy *gatewayv1.TokenQuotaPolicy) {
	id := policy.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindTokenQuotaPolicy, id, ReasonInvalidSpec, "token quota policy metadata.name is required")
		return
	}
	if _, ok := c.tokenQuotaPolicies[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindTokenQuotaPolicy, id, ReasonConflict, fmt.Sprintf("duplicate token quota policy %q", id))
		return
	}
	c.tokenQuotaPolicies[id] = policy
}

func (c *compilation) addDiagnostic(severity Severity, kind gatewayv1.Kind, id string, reason Reason, message string) {
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

func (c *compilation) sortDiagnostics() {
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

func sortEnvoyConfig(config *EnvoyConfig) {
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
