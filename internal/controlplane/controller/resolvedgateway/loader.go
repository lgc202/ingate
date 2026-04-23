package resolvedgateway

import (
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	controllerruntime "github.com/lgc202/ingate/internal/controlplane/controller/runtime"
	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	gatewaylisters "github.com/lgc202/ingate/pkg/generated/listers/gateway/v1alpha1"
	policylisters "github.com/lgc202/ingate/pkg/generated/listers/policy/v1alpha1"
)

type Loader struct {
	gateways        gatewaylisters.GatewayLister
	routes          gatewaylisters.RouteLister
	backends        gatewaylisters.BackendLister
	certificates    gatewaylisters.CertificateLister
	authPolicies    policylisters.AuthPolicyLister
	trafficPolicies policylisters.TrafficPolicyLister
}

func NewLoader(ctx *controllerruntime.Context) *Loader {
	if ctx == nil || ctx.InformerFactory == nil {
		return &Loader{}
	}
	gatewayInformers := ctx.InformerFactory.Gateway().V1alpha1()
	policyInformers := ctx.InformerFactory.Policy().V1alpha1()
	return &Loader{
		gateways:        gatewayInformers.Gateways().Lister(),
		routes:          gatewayInformers.Routes().Lister(),
		backends:        gatewayInformers.Backends().Lister(),
		certificates:    gatewayInformers.Certificates().Lister(),
		authPolicies:    policyInformers.AuthPolicies().Lister(),
		trafficPolicies: policyInformers.TrafficPolicies().Lister(),
	}
}

func (l *Loader) Load(gatewayKey shared.ObjectKey) (*ResourceBundle, error) {
	if l == nil || l.gateways == nil {
		return nil, fmt.Errorf("resolvedgateway loader is not initialized")
	}
	gateway, err := l.gateways.Get(gatewayKey.Name)
	if err != nil {
		return nil, err
	}
	if !matchesNamespace(gateway.Namespace, gatewayKey.Namespace) {
		return nil, apierrors.NewNotFound(gatewayv1alpha1.Resource("gateway"), gatewayKey.String())
	}

	bundle := &ResourceBundle{Gateway: gateway}
	bundle.Routes, err = l.loadRoutes(gateway)
	if err != nil {
		return nil, err
	}
	bundle.Backends, err = l.loadBackends(gateway, bundle.Routes)
	if err != nil {
		return nil, err
	}
	bundle.Certificates, err = l.loadCertificates(gateway)
	if err != nil {
		return nil, err
	}
	bundle.GatewayAuthPolicies, bundle.RouteAuthPolicies, bundle.BackendAuthPolicies, err = l.loadAuthPolicies(gateway, bundle.Routes, bundle.Backends)
	if err != nil {
		return nil, err
	}
	bundle.GatewayTrafficPolicies, bundle.RouteTrafficPolicies, bundle.BackendTrafficPolicies, err = l.loadTrafficPolicies(gateway, bundle.Routes, bundle.Backends)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}

func (l *Loader) loadRoutes(gateway *gatewayv1alpha1.Gateway) ([]*gatewayv1alpha1.Route, error) {
	items, err := l.routes.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var routes []*gatewayv1alpha1.Route
	for _, route := range items {
		if route == nil || !matchesNamespace(route.Namespace, gateway.Namespace) {
			continue
		}
		for _, parent := range route.Spec.ParentRefs {
			if parent.Name == gateway.Name {
				routes = append(routes, route)
				break
			}
		}
	}
	return routes, nil
}

func (l *Loader) loadBackends(gateway *gatewayv1alpha1.Gateway, routes []*gatewayv1alpha1.Route) ([]*gatewayv1alpha1.Backend, error) {
	seen := map[string]struct{}{}
	var backends []*gatewayv1alpha1.Backend
	for _, route := range routes {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.BackendRefs {
				if ref.Name == "" {
					continue
				}
				if _, ok := seen[ref.Name]; ok {
					continue
				}
				backend, err := l.backends.Get(ref.Name)
				if err != nil {
					return nil, err
				}
				if !matchesNamespace(backend.Namespace, gateway.Namespace) {
					continue
				}
				seen[ref.Name] = struct{}{}
				backends = append(backends, backend)
			}
		}
	}
	return backends, nil
}

func (l *Loader) loadCertificates(gateway *gatewayv1alpha1.Gateway) ([]*gatewayv1alpha1.Certificate, error) {
	seen := map[string]struct{}{}
	var certificates []*gatewayv1alpha1.Certificate
	for _, listener := range gateway.Spec.Listeners {
		if listener.TLS == nil || listener.TLS.CertificateRef == nil || listener.TLS.CertificateRef.Name == "" {
			continue
		}
		name := listener.TLS.CertificateRef.Name
		if _, ok := seen[name]; ok {
			continue
		}
		certificate, err := l.certificates.Get(name)
		if err != nil {
			return nil, err
		}
		if !matchesNamespace(certificate.Namespace, gateway.Namespace) {
			continue
		}
		seen[name] = struct{}{}
		certificates = append(certificates, certificate)
	}
	return certificates, nil
}

func (l *Loader) loadAuthPolicies(gateway *gatewayv1alpha1.Gateway, routes []*gatewayv1alpha1.Route, backends []*gatewayv1alpha1.Backend) ([]*policyv1alpha1.AuthPolicy, map[string][]*policyv1alpha1.AuthPolicy, map[string][]*policyv1alpha1.AuthPolicy, error) {
	items, err := l.authPolicies.List(labels.Everything())
	if err != nil {
		return nil, nil, nil, err
	}
	routeNames := namesOfRoutes(routes)
	backendNames := namesOfBackends(backends)
	gatewayPolicies := []*policyv1alpha1.AuthPolicy{}
	routePolicies := map[string][]*policyv1alpha1.AuthPolicy{}
	backendPolicies := map[string][]*policyv1alpha1.AuthPolicy{}
	for _, policy := range items {
		if policy == nil || !matchesNamespace(policy.Namespace, gateway.Namespace) {
			continue
		}
		matchedGateway := false
		matchedRoutes := map[string]struct{}{}
		matchedBackends := map[string]struct{}{}
		for _, target := range policy.Spec.TargetRefs {
			switch target.Kind {
			case "Gateway":
				if target.Name == gateway.Name {
					matchedGateway = true
				}
			case "Route":
				routeKey := shared.NewObjectKey(policy.Namespace, target.Name).String()
				if _, ok := routeNames[routeKey]; ok {
					matchedRoutes[routeKey] = struct{}{}
				}
			case "Backend":
				backendKey := shared.NewObjectKey(policy.Namespace, target.Name).String()
				if _, ok := backendNames[backendKey]; ok {
					matchedBackends[backendKey] = struct{}{}
				}
			}
		}
		if matchedGateway {
			gatewayPolicies = append(gatewayPolicies, policy)
		}
		for routeKey := range matchedRoutes {
			routePolicies[routeKey] = append(routePolicies[routeKey], policy)
		}
		for backendKey := range matchedBackends {
			backendPolicies[backendKey] = append(backendPolicies[backendKey], policy)
		}
	}
	return gatewayPolicies, routePolicies, backendPolicies, nil
}

func (l *Loader) loadTrafficPolicies(gateway *gatewayv1alpha1.Gateway, routes []*gatewayv1alpha1.Route, backends []*gatewayv1alpha1.Backend) ([]*policyv1alpha1.TrafficPolicy, map[string][]*policyv1alpha1.TrafficPolicy, map[string][]*policyv1alpha1.TrafficPolicy, error) {
	items, err := l.trafficPolicies.List(labels.Everything())
	if err != nil {
		return nil, nil, nil, err
	}
	routeNames := namesOfRoutes(routes)
	backendNames := namesOfBackends(backends)
	gatewayPolicies := []*policyv1alpha1.TrafficPolicy{}
	routePolicies := map[string][]*policyv1alpha1.TrafficPolicy{}
	backendPolicies := map[string][]*policyv1alpha1.TrafficPolicy{}
	for _, policy := range items {
		if policy == nil || !matchesNamespace(policy.Namespace, gateway.Namespace) {
			continue
		}
		matchedGateway := false
		matchedRoutes := map[string]struct{}{}
		matchedBackends := map[string]struct{}{}
		for _, target := range policy.Spec.TargetRefs {
			switch target.Kind {
			case "Gateway":
				if target.Name == gateway.Name {
					matchedGateway = true
				}
			case "Route":
				routeKey := shared.NewObjectKey(policy.Namespace, target.Name).String()
				if _, ok := routeNames[routeKey]; ok {
					matchedRoutes[routeKey] = struct{}{}
				}
			case "Backend":
				backendKey := shared.NewObjectKey(policy.Namespace, target.Name).String()
				if _, ok := backendNames[backendKey]; ok {
					matchedBackends[backendKey] = struct{}{}
				}
			}
		}
		if matchedGateway {
			gatewayPolicies = append(gatewayPolicies, policy)
		}
		for routeKey := range matchedRoutes {
			routePolicies[routeKey] = append(routePolicies[routeKey], policy)
		}
		for backendKey := range matchedBackends {
			backendPolicies[backendKey] = append(backendPolicies[backendKey], policy)
		}
	}
	return gatewayPolicies, routePolicies, backendPolicies, nil
}

func matchesNamespace(objectNamespace, expectedNamespace string) bool {
	return objectNamespace == "" || expectedNamespace == "" || objectNamespace == expectedNamespace
}

func namesOfRoutes(routes []*gatewayv1alpha1.Route) map[string]struct{} {
	items := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		if route != nil && route.Name != "" {
			items[shared.NewObjectKey(route.Namespace, route.Name).String()] = struct{}{}
		}
	}
	return items
}

func namesOfBackends(backends []*gatewayv1alpha1.Backend) map[string]struct{} {
	items := make(map[string]struct{}, len(backends))
	for _, backend := range backends {
		if backend != nil && backend.Name != "" {
			items[shared.NewObjectKey(backend.Namespace, backend.Name).String()] = struct{}{}
		}
	}
	return items
}
