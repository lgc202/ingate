package gatewaycompiler

import (
	"fmt"
	"sort"

	compilerir "github.com/lgc202/ingate/internal/controlplane/compiler/ir"
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

func BuildLogicalGateway(bundle *ResourceBundle) (*compilerir.LogicalGateway, error) {
	if bundle == nil || bundle.Gateway == nil {
		return nil, fmt.Errorf("resource bundle must include a gateway")
	}

	certificateMap := make(map[string]*gatewayv1alpha1.Certificate, len(bundle.Certificates))
	for _, certificate := range bundle.Certificates {
		if certificate == nil || certificate.Name == "" {
			continue
		}
		certificateMap[objectKeyOf(certificate).String()] = certificate
	}

	routes, routeIndexes := buildRoutes(bundle.Routes)
	backends, backendIndexes := buildBackends(bundle.Backends)

	for routeKey, policies := range bundle.RouteAuthPolicies {
		if index, ok := routeIndexes[routeKey]; ok {
			routes[index].AuthSummary = buildAuthSummary(policies)
		}
	}
	for routeKey, policies := range bundle.RouteTrafficPolicies {
		if index, ok := routeIndexes[routeKey]; ok {
			routes[index].TrafficSummary = buildTrafficSummary(policies)
		}
	}
	for backendKey, policies := range bundle.BackendAuthPolicies {
		if index, ok := backendIndexes[backendKey]; ok {
			backends[index].AuthSummary = buildAuthSummary(policies)
		}
	}
	for backendKey, policies := range bundle.BackendTrafficPolicies {
		if index, ok := backendIndexes[backendKey]; ok {
			backends[index].TrafficSummary = buildTrafficSummary(policies)
		}
	}

	return &compilerir.LogicalGateway{
		Meta: compilerir.GatewayMeta{
			Namespace: bundle.Gateway.Namespace,
			Name:      bundle.Gateway.Name,
			Version:   bundle.Gateway.ResourceVersion,
		},
		Listeners: buildListeners(bundle.Gateway, certificateMap),
		Routes:    routes,
		Backends:  backends,
		Policies: compilerir.Policies{
			GatewayAuth:    buildAuthSummary(bundle.GatewayAuthPolicies),
			GatewayTraffic: buildTrafficSummary(bundle.GatewayTrafficPolicies),
		},
		Trace: traceInfoFromBundle(bundle),
	}, nil
}

func buildListeners(gateway *gatewayv1alpha1.Gateway, certificates map[string]*gatewayv1alpha1.Certificate) []compilerir.Listener {
	listeners := make([]compilerir.Listener, 0, len(gateway.Spec.Listeners))
	for _, listener := range gateway.Spec.Listeners {
		item := compilerir.Listener{
			Name:      listener.Name,
			Protocol:  listener.Protocol,
			Port:      listener.Port,
			Hostnames: collectListenerHostnames(listener),
		}
		if listener.TLS != nil {
			item.TLS = &compilerir.ListenerTLS{Mode: listener.TLS.Mode}
			if listener.TLS.CertificateRef != nil {
				item.TLS.CertificateName = listener.TLS.CertificateRef.Name
				if certificate := certificates[shared.NewObjectKey(gateway.Namespace, listener.TLS.CertificateRef.Name).String()]; certificate != nil {
					item.TLS.SecretName = certificate.Spec.SecretRef.Name
					item.TLS.Domains = append([]string(nil), certificate.Spec.Domains...)
				}
			}
		}
		listeners = append(listeners, item)
	}
	return listeners
}

func buildRoutes(routes []*gatewayv1alpha1.Route) ([]compilerir.Route, map[string]int) {
	items := make([]compilerir.Route, 0, len(routes))
	indexes := make(map[string]int, len(routes))
	for _, route := range routes {
		if route == nil {
			continue
		}
		indexes[objectKeyOf(route).String()] = len(items)
		items = append(items, compilerir.Route{
			Name:      route.Name,
			Hostnames: append([]string(nil), route.Spec.Hostnames...),
			Rules:     buildRouteRules(route.Spec.Rules),
		})
	}
	return items, indexes
}

func buildRouteRules(rules []gatewayv1alpha1.RouteRule) []compilerir.RouteRule {
	items := make([]compilerir.RouteRule, 0, len(rules))
	for _, rule := range rules {
		item := compilerir.RouteRule{
			Matches:     append([]gatewayv1alpha1.HTTPRouteMatch(nil), rule.Matches...),
			BackendRefs: append([]gatewayv1alpha1.BackendRef(nil), rule.BackendRefs...),
		}
		if len(rule.Filters) > 0 {
			item.Filters = make([]gatewayv1alpha1.HTTPRouteFilter, 0, len(rule.Filters))
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
				item.Filters = append(item.Filters, filterCopy)
			}
		}
		items = append(items, item)
	}
	return items
}

func buildBackends(backends []*gatewayv1alpha1.Backend) ([]compilerir.Backend, map[string]int) {
	items := make([]compilerir.Backend, 0, len(backends))
	indexes := make(map[string]int, len(backends))
	for _, backend := range backends {
		if backend == nil {
			continue
		}
		item := compilerir.Backend{
			Name:        backend.Name,
			Protocol:    backend.Spec.Protocol,
			DefaultPort: backend.Spec.DefaultPort,
			LoadBalance: cloneLoadBalance(backend.Spec.LoadBalance),
		}
		if backend.Spec.Static != nil && len(backend.Spec.Static.Endpoints) > 0 {
			item.Endpoints = append([]gatewayv1alpha1.BackendEndpoint(nil), backend.Spec.Static.Endpoints...)
		}
		if len(item.Endpoints) == 0 {
			item.Endpoints = append([]gatewayv1alpha1.BackendEndpoint(nil), backend.Status.Endpoints...)
		}
		indexes[objectKeyOf(backend).String()] = len(items)
		items = append(items, item)
	}
	return items, indexes
}

func buildAuthSummary(policies []*policyv1alpha1.AuthPolicy) *compilerir.AuthSummary {
	if len(policies) == 0 {
		return nil
	}
	itemsByName := map[string]compilerir.PolicyRef{}
	for _, policy := range policies {
		if policy == nil || policy.Name == "" {
			continue
		}
		itemsByName[policy.Name] = compilerir.PolicyRef{Kind: "AuthPolicy", Name: policy.Name, Type: policy.Spec.Type}
	}
	if len(itemsByName) == 0 {
		return nil
	}
	policyNames := make([]string, 0, len(itemsByName))
	for name := range itemsByName {
		policyNames = append(policyNames, name)
	}
	sort.Strings(policyNames)
	items := make([]compilerir.PolicyRef, 0, len(policyNames))
	for _, name := range policyNames {
		items = append(items, itemsByName[name])
	}
	return &compilerir.AuthSummary{Policies: items}
}

func buildTrafficSummary(policies []*policyv1alpha1.TrafficPolicy) *compilerir.TrafficSummary {
	if len(policies) == 0 {
		return nil
	}
	itemsByName := map[string]compilerir.TrafficPolicyRef{}
	for _, policy := range policies {
		if policy == nil || policy.Name == "" {
			continue
		}
		item := compilerir.TrafficPolicyRef{
			Kind: "TrafficPolicy",
			Name: policy.Name,
		}
		if policy.Spec.Timeout != nil {
			item.TimeoutDuration = policy.Spec.Timeout.Duration
		}
		if policy.Spec.Retry != nil {
			item.RetryAttempts = policy.Spec.Retry.Attempts
			item.RetryConditions = append([]string(nil), policy.Spec.Retry.Conditions...)
		}
		if policy.Spec.RateLimit != nil {
			item.RateLimitRequests = policy.Spec.RateLimit.RequestsPerUnit
			item.RateLimitUnit = policy.Spec.RateLimit.Unit
			item.RateLimitScope = policy.Spec.RateLimit.Scope
		}
		itemsByName[policy.Name] = item
	}
	if len(itemsByName) == 0 {
		return nil
	}
	policyNames := make([]string, 0, len(itemsByName))
	for name := range itemsByName {
		policyNames = append(policyNames, name)
	}
	sort.Strings(policyNames)
	items := make([]compilerir.TrafficPolicyRef, 0, len(policyNames))
	for _, name := range policyNames {
		items = append(items, itemsByName[name])
	}
	return &compilerir.TrafficSummary{Policies: items}
}

func traceInfoFromBundle(bundle *ResourceBundle) *compilerir.TraceInfo {
	if bundle == nil {
		return nil
	}

	sources := make([]compilerir.SourceRef, 0, 1+len(bundle.Routes)+len(bundle.Backends)+len(bundle.Certificates)+len(bundle.GatewayAuthPolicies)+len(bundle.GatewayTrafficPolicies))
	appendSource := func(kind string, namespace string, name string) {
		if name == "" {
			return
		}
		sources = append(sources, compilerir.SourceRef{Kind: kind, Namespace: namespace, Name: name})
	}

	if bundle.Gateway != nil {
		appendSource("Gateway", bundle.Gateway.Namespace, bundle.Gateway.Name)
	}
	for _, route := range bundle.Routes {
		if route != nil {
			appendSource("Route", route.Namespace, route.Name)
		}
	}
	for _, backend := range bundle.Backends {
		if backend != nil {
			appendSource("Backend", backend.Namespace, backend.Name)
		}
	}
	for _, certificate := range bundle.Certificates {
		if certificate != nil {
			appendSource("Certificate", certificate.Namespace, certificate.Name)
		}
	}
	for _, policy := range collectAuthPolicies(bundle) {
		if policy != nil {
			appendSource("AuthPolicy", policy.Namespace, policy.Name)
		}
	}
	for _, policy := range collectTrafficPolicies(bundle) {
		if policy != nil {
			appendSource("TrafficPolicy", policy.Namespace, policy.Name)
		}
	}
	if len(sources) == 0 {
		return nil
	}
	return &compilerir.TraceInfo{Sources: sources}
}

func cloneLoadBalance(loadBalance *gatewayv1alpha1.LoadBalanceSpec) *gatewayv1alpha1.LoadBalanceSpec {
	if loadBalance == nil {
		return nil
	}
	copied := *loadBalance
	return &copied
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
