package resolvedgateway

import (
	"fmt"
	"sort"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

type ResourceBundle struct {
	Gateway      *gatewayv1alpha1.Gateway
	Routes       []*gatewayv1alpha1.Route
	Backends     []*gatewayv1alpha1.Backend
	Certificates []*gatewayv1alpha1.Certificate

	GatewayAuthPolicies    []*policyv1alpha1.AuthPolicy
	RouteAuthPolicies      map[string][]*policyv1alpha1.AuthPolicy
	BackendAuthPolicies    map[string][]*policyv1alpha1.AuthPolicy
	GatewayTrafficPolicies []*policyv1alpha1.TrafficPolicy
	RouteTrafficPolicies   map[string][]*policyv1alpha1.TrafficPolicy
	BackendTrafficPolicies map[string][]*policyv1alpha1.TrafficPolicy
}

func Build(bundle *ResourceBundle) (*gatewayv1alpha1.ResolvedGateway, error) {
	if bundle == nil || bundle.Gateway == nil {
		return nil, fmt.Errorf("resource bundle must include a gateway")
	}

	gateway := bundle.Gateway
	certificates := make(map[string]*gatewayv1alpha1.Certificate, len(bundle.Certificates))
	for _, certificate := range bundle.Certificates {
		if certificate == nil || certificate.Name == "" {
			continue
		}
		certificates[shared.NewObjectKey(certificate.Namespace, certificate.Name).String()] = certificate
	}

	resolved := &gatewayv1alpha1.ResolvedGateway{}
	resolved.Name = gateway.Name
	resolved.Spec.GatewayRef = gatewayv1alpha1.LocalObjectReference{Name: gateway.Name}
	resolved.Spec.Version = gateway.ResourceVersion
	resolved.Spec.GatewayAuthSummary = buildAuthSummary(bundle.GatewayAuthPolicies)
	resolved.Spec.GatewayTrafficSummary = buildTrafficSummary(bundle.GatewayTrafficPolicies)

	resolved.Spec.Listeners = buildListeners(gateway, certificates)
	resolved.Spec.Routes = buildRoutes(bundle)
	resolved.Spec.Backends = buildBackends(bundle)
	resolved.Spec.Extensions = []gatewayv1alpha1.ResolvedGatewayExtension{}

	return resolved, nil
}

func buildListeners(gateway *gatewayv1alpha1.Gateway, certificates map[string]*gatewayv1alpha1.Certificate) []gatewayv1alpha1.ResolvedGatewayListener {
	if gateway == nil {
		return nil
	}

	listeners := make([]gatewayv1alpha1.ResolvedGatewayListener, 0, len(gateway.Spec.Listeners))
	for _, listener := range gateway.Spec.Listeners {
		resolved := gatewayv1alpha1.ResolvedGatewayListener{
			Name:      listener.Name,
			Protocol:  listener.Protocol,
			Port:      listener.Port,
			Hostnames: collectListenerHostnames(listener),
		}
		if listener.TLS != nil {
			resolvedTLS := &gatewayv1alpha1.ResolvedGatewayListenerTLS{Mode: listener.TLS.Mode}
			if listener.TLS.CertificateRef != nil {
				ref := *listener.TLS.CertificateRef
				resolvedTLS.CertificateRef = &ref
				if certificate := certificates[shared.NewObjectKey(gateway.Namespace, listener.TLS.CertificateRef.Name).String()]; certificate != nil {
					secretRef := certificate.Spec.SecretRef
					resolvedTLS.SecretRef = &secretRef
					resolvedTLS.Domains = append([]string(nil), certificate.Spec.Domains...)
				}
			}
			resolved.TLS = resolvedTLS
		}
		listeners = append(listeners, resolved)
	}
	return listeners
}

func buildRoutes(bundle *ResourceBundle) []gatewayv1alpha1.ResolvedGatewayRoute {
	if bundle == nil {
		return nil
	}
	items := make([]gatewayv1alpha1.ResolvedGatewayRoute, 0, len(bundle.Routes))
	for _, route := range bundle.Routes {
		if route == nil {
			continue
		}
		routeKey := objectKeyOf(route).String()
		resolved := gatewayv1alpha1.ResolvedGatewayRoute{
			Name:           route.Name,
			Hostnames:      append([]string(nil), route.Spec.Hostnames...),
			Rules:          buildRouteRules(route.Spec.Rules),
			AuthSummary:    buildAuthSummary(bundle.RouteAuthPolicies[routeKey]),
			TrafficSummary: buildTrafficSummary(bundle.RouteTrafficPolicies[routeKey]),
		}
		items = append(items, resolved)
	}
	return items
}

func buildRouteRules(rules []gatewayv1alpha1.RouteRule) []gatewayv1alpha1.ResolvedGatewayRouteRule {
	items := make([]gatewayv1alpha1.ResolvedGatewayRouteRule, 0, len(rules))
	for _, rule := range rules {
		resolved := gatewayv1alpha1.ResolvedGatewayRouteRule{
			Matches:     append([]gatewayv1alpha1.HTTPRouteMatch(nil), rule.Matches...),
			BackendRefs: append([]gatewayv1alpha1.BackendRef(nil), rule.BackendRefs...),
		}
		if len(rule.Filters) > 0 {
			resolved.Filters = make([]gatewayv1alpha1.HTTPRouteFilter, 0, len(rule.Filters))
			for _, filter := range rule.Filters {
				filterCopy := filter
				if filter.URLRewrite != nil {
					filterCopy.URLRewrite = filter.URLRewrite.DeepCopy()
				}
				if filter.RequestHeaderModifier != nil {
					filterCopy.RequestHeaderModifier = filter.RequestHeaderModifier.DeepCopy()
				}
				if filter.ResponseHeaderModifier != nil {
					filterCopy.ResponseHeaderModifier = filter.ResponseHeaderModifier.DeepCopy()
				}
				resolved.Filters = append(resolved.Filters, filterCopy)
			}
		}
		items = append(items, resolved)
	}
	return items
}

func buildBackends(bundle *ResourceBundle) []gatewayv1alpha1.ResolvedGatewayBackend {
	if bundle == nil {
		return nil
	}
	items := make([]gatewayv1alpha1.ResolvedGatewayBackend, 0, len(bundle.Backends))
	for _, backend := range bundle.Backends {
		if backend == nil {
			continue
		}
		backendKey := objectKeyOf(backend).String()
		resolved := gatewayv1alpha1.ResolvedGatewayBackend{
			Name:           backend.Name,
			Protocol:       backend.Spec.Protocol,
			DefaultPort:    backend.Spec.DefaultPort,
			AuthSummary:    buildAuthSummary(bundle.BackendAuthPolicies[backendKey]),
			TrafficSummary: buildTrafficSummary(bundle.BackendTrafficPolicies[backendKey]),
		}
		if backend.Spec.LoadBalance != nil {
			loadBalance := *backend.Spec.LoadBalance
			resolved.LoadBalance = &loadBalance
		}
		if backend.Spec.Static != nil && len(backend.Spec.Static.Endpoints) > 0 {
			resolved.Endpoints = append([]gatewayv1alpha1.BackendEndpoint(nil), backend.Spec.Static.Endpoints...)
		}
		if len(resolved.Endpoints) == 0 {
			resolved.Endpoints = append([]gatewayv1alpha1.BackendEndpoint(nil), backend.Status.Endpoints...)
		}
		items = append(items, resolved)
	}
	return items
}

func buildAuthSummary(policies []*policyv1alpha1.AuthPolicy) *gatewayv1alpha1.ResolvedGatewayAuthSummary {
	if len(policies) == 0 {
		return nil
	}
	deduped := map[string]gatewayv1alpha1.ResolvedPolicyRef{}
	for _, policy := range policies {
		if policy == nil || policy.Name == "" {
			continue
		}
		deduped[policy.Name] = gatewayv1alpha1.ResolvedPolicyRef{Kind: "AuthPolicy", Name: policy.Name, Type: policy.Spec.Type}
	}
	if len(deduped) == 0 {
		return nil
	}
	names := make([]string, 0, len(deduped))
	for name := range deduped {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]gatewayv1alpha1.ResolvedPolicyRef, 0, len(names))
	for _, name := range names {
		items = append(items, deduped[name])
	}
	return &gatewayv1alpha1.ResolvedGatewayAuthSummary{Policies: items}
}

func buildTrafficSummary(policies []*policyv1alpha1.TrafficPolicy) *gatewayv1alpha1.ResolvedGatewayTrafficSummary {
	if len(policies) == 0 {
		return nil
	}
	deduped := map[string]gatewayv1alpha1.ResolvedTrafficPolicyRef{}
	for _, policy := range policies {
		if policy == nil || policy.Name == "" {
			continue
		}
		ref := gatewayv1alpha1.ResolvedTrafficPolicyRef{
			Kind: "TrafficPolicy",
			Name: policy.Name,
		}
		if policy.Spec.Timeout != nil {
			ref.TimeoutDuration = policy.Spec.Timeout.Duration
		}
		if policy.Spec.Retry != nil {
			ref.RetryAttempts = policy.Spec.Retry.Attempts
			ref.RetryConditions = append([]string(nil), policy.Spec.Retry.Conditions...)
		}
		if policy.Spec.RateLimit != nil {
			ref.RateLimitRequests = policy.Spec.RateLimit.RequestsPerUnit
			ref.RateLimitUnit = policy.Spec.RateLimit.Unit
			ref.RateLimitScope = policy.Spec.RateLimit.Scope
		}
		deduped[policy.Name] = ref
	}
	if len(deduped) == 0 {
		return nil
	}
	names := make([]string, 0, len(deduped))
	for name := range deduped {
		names = append(names, name)
	}
	sort.Strings(names)
	summary := &gatewayv1alpha1.ResolvedGatewayTrafficSummary{Policies: make([]gatewayv1alpha1.ResolvedTrafficPolicyRef, 0, len(names))}
	for _, name := range names {
		summary.Policies = append(summary.Policies, deduped[name])
	}
	return summary
}

func collectListenerHostnames(listener gatewayv1alpha1.GatewayListener) []string {
	seen := make(map[string]struct{}, len(listener.Hostnames)+1)
	items := make([]string, 0, len(listener.Hostnames)+1)
	appendHostname := func(host string) {
		if host == "" {
			return
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		items = append(items, host)
	}
	appendHostname(listener.Hostname)
	for _, host := range listener.Hostnames {
		appendHostname(host)
	}
	sort.Strings(items)
	return items
}

func objectKeyOf(obj interface {
	GetNamespace() string
	GetName() string
}) shared.ObjectKey {
	return shared.NewObjectKey(obj.GetNamespace(), obj.GetName())
}
