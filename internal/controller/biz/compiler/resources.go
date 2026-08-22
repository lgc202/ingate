package compiler

import (
	"fmt"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

type compilation struct {
	gateways                     map[string]*gatewayv1.Gateway
	certificates                 map[string]*gatewayv1.Certificate
	validCertificates            map[string]bool
	routes                       map[string]*gatewayv1.Route
	upstreams                    map[string]*gatewayv1.Upstream
	rateLimitPolicies            map[string]*gatewayv1.RateLimitPolicy
	ipRestrictionPolicies        map[string]*gatewayv1.IPRestrictionPolicy
	headerTransformationPolicies map[string]*gatewayv1.HeaderTransformationPolicy
	wasmPlugins                  map[string]*gatewayv1.WasmPlugin
	wasmPluginsByPackage         map[string]*gatewayv1.WasmPlugin
	wasmModules                  map[string]WasmModule

	diagnostics   []Diagnostic
	diagnosticSet map[string]bool
}

func newCompilation(resources Resources, wasmModules map[string]WasmModule) *compilation {
	return &compilation{
		gateways:                     make(map[string]*gatewayv1.Gateway, len(resources.Gateways)),
		certificates:                 make(map[string]*gatewayv1.Certificate, len(resources.Certificates)),
		validCertificates:            make(map[string]bool, len(resources.Certificates)),
		routes:                       make(map[string]*gatewayv1.Route, len(resources.Routes)),
		upstreams:                    make(map[string]*gatewayv1.Upstream, len(resources.Upstreams)),
		rateLimitPolicies:            make(map[string]*gatewayv1.RateLimitPolicy, len(resources.RateLimitPolicies)),
		ipRestrictionPolicies:        make(map[string]*gatewayv1.IPRestrictionPolicy, len(resources.IPRestrictionPolicies)),
		headerTransformationPolicies: make(map[string]*gatewayv1.HeaderTransformationPolicy, len(resources.HeaderTransformationPolicies)),
		wasmPlugins:                  make(map[string]*gatewayv1.WasmPlugin, len(resources.WasmPlugins)),
		wasmPluginsByPackage:         make(map[string]*gatewayv1.WasmPlugin, len(resources.WasmPlugins)),
		wasmModules:                  wasmModules,
		diagnosticSet:                make(map[string]bool),
	}
}

func (c *compilation) indexResources(resources Resources) {
	for _, certificate := range resources.Certificates {
		if certificate == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindCertificate, "", ReasonInvalidSpec, "certificate resource is nil")
			continue
		}
		c.indexCertificate(certificate)
	}
	for _, gateway := range resources.Gateways {
		if gateway == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindGateway, "", ReasonInvalidSpec, "gateway resource is nil")
			continue
		}
		c.indexGateway(gateway)
	}
	for _, route := range resources.Routes {
		if route == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, "", ReasonInvalidSpec, "route resource is nil")
			continue
		}
		c.indexRoute(route)
	}
	for _, upstream := range resources.Upstreams {
		if upstream == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindUpstream, "", ReasonInvalidSpec, "upstream resource is nil")
			continue
		}
		c.indexUpstream(upstream)
	}
	for _, policy := range resources.RateLimitPolicies {
		if policy == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindRateLimitPolicy, "", ReasonInvalidSpec, "rate limit policy resource is nil")
			continue
		}
		c.indexRateLimitPolicy(policy)
	}
	for _, policy := range resources.IPRestrictionPolicies {
		if policy == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindIPRestrictionPolicy, "", ReasonInvalidSpec, "IP restriction policy resource is nil")
			continue
		}
		c.indexIPRestrictionPolicy(policy)
	}
	for _, policy := range resources.HeaderTransformationPolicies {
		if policy == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindHeaderTransformationPolicy, "", ReasonInvalidSpec, "header transformation policy resource is nil")
			continue
		}
		c.indexHeaderTransformationPolicy(policy)
	}
	for _, plugin := range resources.WasmPlugins {
		if plugin == nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindWasmPlugin, "", ReasonInvalidSpec, "Wasm plugin resource is nil")
			continue
		}
		c.indexWasmPlugin(plugin)
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

func (c *compilation) indexIPRestrictionPolicy(policy *gatewayv1.IPRestrictionPolicy) {
	id := policy.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindIPRestrictionPolicy, id, ReasonInvalidSpec, "IP restriction policy metadata.name is required")
		return
	}
	if _, ok := c.ipRestrictionPolicies[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindIPRestrictionPolicy, id, ReasonConflict, fmt.Sprintf("duplicate IP restriction policy %q", id))
		return
	}
	c.ipRestrictionPolicies[id] = policy
}

func (c *compilation) indexWasmPlugin(plugin *gatewayv1.WasmPlugin) {
	id := plugin.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindWasmPlugin, id, ReasonInvalidSpec, "Wasm plugin metadata.name is required")
		return
	}
	if _, ok := c.wasmPlugins[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindWasmPlugin, id, ReasonConflict, fmt.Sprintf("duplicate Wasm plugin %q", id))
		return
	}
	c.wasmPlugins[id] = plugin
	if existing, ok := c.wasmPluginsByPackage[plugin.Spec.Package]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindWasmPlugin, id, ReasonConflict, fmt.Sprintf("Wasm plugin package %q is already installed as %q", plugin.Spec.Package, existing.Name))
		return
	}
	c.wasmPluginsByPackage[plugin.Spec.Package] = plugin
}

func (c *compilation) indexHeaderTransformationPolicy(policy *gatewayv1.HeaderTransformationPolicy) {
	id := policy.Name
	if id == "" {
		c.addDiagnostic(SeverityError, gatewayv1.KindHeaderTransformationPolicy, id, ReasonInvalidSpec, "header transformation policy metadata.name is required")
		return
	}
	if _, ok := c.headerTransformationPolicies[id]; ok {
		c.addDiagnostic(SeverityError, gatewayv1.KindHeaderTransformationPolicy, id, ReasonConflict, fmt.Sprintf("duplicate header transformation policy %q", id))
		return
	}
	c.headerTransformationPolicies[id] = policy
}
