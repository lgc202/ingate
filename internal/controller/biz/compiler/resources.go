package compiler

import (
	"crypto/x509"
	"fmt"
	"time"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

type compilation struct {
	gateways                     map[string]*gatewayv1.Gateway
	certificates                 map[string]*gatewayv1.Certificate
	certificateLeaves            map[string]*x509.Certificate
	routes                       map[string]*gatewayv1.Route
	upstreams                    map[string]*gatewayv1.Upstream
	rateLimitPolicies            map[string]*gatewayv1.RateLimitPolicy
	ipRestrictionPolicies        map[string]*gatewayv1.IPRestrictionPolicy
	headerTransformationPolicies map[string]*gatewayv1.HeaderTransformationPolicy
	mockResponsePolicies         map[string]*gatewayv1.MockResponsePolicy
	wasmPlugins                  map[string]*gatewayv1.WasmPlugin
	wasmPluginsByPackage         map[string]*gatewayv1.WasmPlugin
	wasmModules                  map[string]WasmModule
	observedAt                   time.Time

	diagnostics   []Diagnostic
	diagnosticSet map[string]bool
}

func newCompilation(
	resources Resources,
	wasmModules map[string]WasmModule,
	observedAt time.Time,
) *compilation {
	return &compilation{
		gateways:              make(map[string]*gatewayv1.Gateway, len(resources.Gateways)),
		certificates:          make(map[string]*gatewayv1.Certificate, len(resources.Certificates)),
		certificateLeaves:     make(map[string]*x509.Certificate, len(resources.Certificates)),
		routes:                make(map[string]*gatewayv1.Route, len(resources.Routes)),
		upstreams:             make(map[string]*gatewayv1.Upstream, len(resources.Upstreams)),
		rateLimitPolicies:     make(map[string]*gatewayv1.RateLimitPolicy, len(resources.RateLimitPolicies)),
		ipRestrictionPolicies: make(map[string]*gatewayv1.IPRestrictionPolicy, len(resources.IPRestrictionPolicies)),
		headerTransformationPolicies: make(
			map[string]*gatewayv1.HeaderTransformationPolicy,
			len(resources.HeaderTransformationPolicies),
		),
		mockResponsePolicies: make(map[string]*gatewayv1.MockResponsePolicy, len(resources.MockResponsePolicies)),
		wasmPlugins:          make(map[string]*gatewayv1.WasmPlugin, len(resources.WasmPlugins)),
		wasmPluginsByPackage: make(map[string]*gatewayv1.WasmPlugin, len(resources.WasmPlugins)),
		wasmModules:          wasmModules,
		observedAt:           observedAt,
		diagnosticSet:        make(map[string]bool),
	}
}

func (c *compilation) indexResources(resources Resources) {
	for _, certificate := range resources.Certificates {
		if certificate == nil {
			c.addKindError(gatewayv1.KindCertificate, ReasonInvalidSpec, "certificate resource is nil")
			continue
		}
		c.indexCertificate(certificate)
	}
	for _, gateway := range resources.Gateways {
		if gateway == nil {
			c.addKindError(gatewayv1.KindGateway, ReasonInvalidSpec, "gateway resource is nil")
			continue
		}
		c.indexGateway(gateway)
	}
	for _, route := range resources.Routes {
		if route == nil {
			c.addKindError(gatewayv1.KindRoute, ReasonInvalidSpec, "route resource is nil")
			continue
		}
		c.indexRoute(route)
	}
	for _, upstream := range resources.Upstreams {
		if upstream == nil {
			c.addKindError(gatewayv1.KindUpstream, ReasonInvalidSpec, "upstream resource is nil")
			continue
		}
		c.indexUpstream(upstream)
	}
	for _, policy := range resources.RateLimitPolicies {
		if policy == nil {
			c.addKindError(
				gatewayv1.KindRateLimitPolicy,
				ReasonInvalidSpec,
				"rate limit policy resource is nil",
			)
			continue
		}
		c.indexRateLimitPolicy(policy)
	}
	for _, policy := range resources.IPRestrictionPolicies {
		if policy == nil {
			c.addKindError(
				gatewayv1.KindIPRestrictionPolicy,
				ReasonInvalidSpec,
				"IP restriction policy resource is nil",
			)
			continue
		}
		c.indexIPRestrictionPolicy(policy)
	}
	for _, policy := range resources.HeaderTransformationPolicies {
		if policy == nil {
			c.addKindError(
				gatewayv1.KindHeaderTransformationPolicy,
				ReasonInvalidSpec,
				"header transformation policy resource is nil",
			)
			continue
		}
		c.indexHeaderTransformationPolicy(policy)
	}
	for _, policy := range resources.MockResponsePolicies {
		if policy == nil {
			c.addKindError(
				gatewayv1.KindMockResponsePolicy,
				ReasonInvalidSpec,
				"mock response policy resource is nil",
			)
			continue
		}
		c.indexMockResponsePolicy(policy)
	}
	for _, plugin := range resources.WasmPlugins {
		if plugin == nil {
			c.addKindError(gatewayv1.KindWasmPlugin, ReasonInvalidSpec, "Wasm plugin resource is nil")
			continue
		}
		c.indexWasmPlugin(plugin)
	}
}

func (c *compilation) indexCertificate(certificate *gatewayv1.Certificate) {
	id := certificate.Name
	if !c.validateResourceID(gatewayv1.KindCertificate, id) {
		return
	}
	if _, ok := c.certificates[id]; ok {
		c.addResourceError(gatewayv1.KindCertificate, id, ReasonConflict, fmt.Sprintf("duplicate certificate %q", id))
		return
	}
	c.certificates[id] = certificate

	if certificate.Spec.CertificatePEM == "" {
		c.addResourceError(
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q certificatePEM is required", id),
		)
		return
	}
	if certificate.Spec.PrivateKeyPEM == "" {
		c.addResourceError(
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q privateKeyPEM is required", id),
		)
		return
	}
	leaf, err := certificateutil.ParseKeyPair(
		certificate.Spec.CertificatePEM,
		certificate.Spec.PrivateKeyPEM,
	)
	if err != nil {
		c.addResourceError(
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q contains an invalid TLS key pair: %v", id, err),
		)
		return
	}
	c.certificateLeaves[id] = leaf
	switch {
	case c.observedAt.Before(leaf.NotBefore):
		c.addResourceWarning(
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q is not valid before %s", id, leaf.NotBefore.UTC().Format(time.RFC3339)),
		)
	case !c.observedAt.Before(leaf.NotAfter):
		c.addResourceWarning(
			gatewayv1.KindCertificate,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("certificate %q expired at %s", id, leaf.NotAfter.UTC().Format(time.RFC3339)),
		)
	}
}

func (c *compilation) indexGateway(gateway *gatewayv1.Gateway) {
	id := gateway.Name
	if !c.validateResourceID(gatewayv1.KindGateway, id) {
		return
	}
	if _, ok := c.gateways[id]; ok {
		c.addResourceError(gatewayv1.KindGateway, id, ReasonConflict, fmt.Sprintf("duplicate gateway %q", id))
		return
	}
	c.gateways[id] = gateway
}

func (c *compilation) indexRoute(route *gatewayv1.Route) {
	id := route.Name
	if !c.validateResourceID(gatewayv1.KindRoute, id) {
		return
	}
	if _, ok := c.routes[id]; ok {
		c.addResourceError(gatewayv1.KindRoute, id, ReasonConflict, fmt.Sprintf("duplicate route %q", id))
		return
	}
	c.routes[id] = route
}

func (c *compilation) indexUpstream(upstream *gatewayv1.Upstream) {
	id := upstream.Name
	if !c.validateResourceID(gatewayv1.KindUpstream, id) {
		return
	}
	if _, ok := c.upstreams[id]; ok {
		c.addResourceError(gatewayv1.KindUpstream, id, ReasonConflict, fmt.Sprintf("duplicate upstream %q", id))
		return
	}
	c.upstreams[id] = upstream
}

func (c *compilation) indexRateLimitPolicy(policy *gatewayv1.RateLimitPolicy) {
	id := policy.Name
	if !c.validateResourceID(gatewayv1.KindRateLimitPolicy, id) {
		return
	}
	if _, ok := c.rateLimitPolicies[id]; ok {
		c.addResourceError(
			gatewayv1.KindRateLimitPolicy,
			id,
			ReasonConflict,
			fmt.Sprintf("duplicate rate limit policy %q", id),
		)
		return
	}
	c.rateLimitPolicies[id] = policy
}

func (c *compilation) indexIPRestrictionPolicy(policy *gatewayv1.IPRestrictionPolicy) {
	id := policy.Name
	if !c.validateResourceID(gatewayv1.KindIPRestrictionPolicy, id) {
		return
	}
	if _, ok := c.ipRestrictionPolicies[id]; ok {
		c.addResourceError(
			gatewayv1.KindIPRestrictionPolicy,
			id,
			ReasonConflict,
			fmt.Sprintf("duplicate IP restriction policy %q", id),
		)
		return
	}
	c.ipRestrictionPolicies[id] = policy
}

func (c *compilation) indexWasmPlugin(plugin *gatewayv1.WasmPlugin) {
	id := plugin.Name
	if !c.validateResourceID(gatewayv1.KindWasmPlugin, id) {
		return
	}
	if _, ok := c.wasmPlugins[id]; ok {
		c.addResourceError(gatewayv1.KindWasmPlugin, id, ReasonConflict, fmt.Sprintf("duplicate Wasm plugin %q", id))
		return
	}
	c.wasmPlugins[id] = plugin
	if existing, ok := c.wasmPluginsByPackage[plugin.Spec.Package]; ok {
		c.addResourceError(
			gatewayv1.KindWasmPlugin,
			id,
			ReasonConflict,
			fmt.Sprintf(
				"Wasm plugin package %q is already installed as %q",
				plugin.Spec.Package,
				existing.Name,
			),
		)
		return
	}
	c.wasmPluginsByPackage[plugin.Spec.Package] = plugin
}

func (c *compilation) indexHeaderTransformationPolicy(policy *gatewayv1.HeaderTransformationPolicy) {
	id := policy.Name
	if !c.validateResourceID(gatewayv1.KindHeaderTransformationPolicy, id) {
		return
	}
	if _, ok := c.headerTransformationPolicies[id]; ok {
		c.addResourceError(
			gatewayv1.KindHeaderTransformationPolicy,
			id,
			ReasonConflict,
			fmt.Sprintf("duplicate header transformation policy %q", id),
		)
		return
	}
	c.headerTransformationPolicies[id] = policy
}

func (c *compilation) indexMockResponsePolicy(policy *gatewayv1.MockResponsePolicy) {
	id := policy.Name
	if !c.validateResourceID(gatewayv1.KindMockResponsePolicy, id) {
		return
	}
	if _, ok := c.mockResponsePolicies[id]; ok {
		c.addResourceError(
			gatewayv1.KindMockResponsePolicy,
			id,
			ReasonConflict,
			fmt.Sprintf("duplicate mock response policy %q", id),
		)
		return
	}
	c.mockResponsePolicies[id] = policy
}

func (c *compilation) validateResourceID(kind gatewayv1.Kind, id string) bool {
	if id == "" {
		c.addResourceError(
			kind,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("%s metadata.name is required", kind),
		)
		return false
	}
	if !resourceconfig.IsCanonicalID(id) {
		c.addResourceError(
			kind,
			id,
			ReasonInvalidSpec,
			fmt.Sprintf("%s %q must use a canonical UUID as metadata.name", kind, id),
		)
		return false
	}
	return true
}
