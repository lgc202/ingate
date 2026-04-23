package index

import (
	"sort"
	"strings"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

type graph struct {
	gateways        map[string]*gatewayv1alpha1.Gateway
	routes          map[string]*gatewayv1alpha1.Route
	backends        map[string]*gatewayv1alpha1.Backend
	certificates    map[string]*gatewayv1alpha1.Certificate
	authPolicies    map[string]*policyv1alpha1.AuthPolicy
	trafficPolicies map[string]*policyv1alpha1.TrafficPolicy

	routeToGateways         map[string]map[string]struct{}
	routeToBackends         map[string]map[string]struct{}
	backendToGateways       map[string]map[string]struct{}
	certificateToGateways   map[string]map[string]struct{}
	authPolicyToGateways    map[string]map[string]struct{}
	trafficPolicyToGateways map[string]map[string]struct{}
}

func newGraph() *graph {
	return &graph{
		gateways:                make(map[string]*gatewayv1alpha1.Gateway),
		routes:                  make(map[string]*gatewayv1alpha1.Route),
		backends:                make(map[string]*gatewayv1alpha1.Backend),
		certificates:            make(map[string]*gatewayv1alpha1.Certificate),
		authPolicies:            make(map[string]*policyv1alpha1.AuthPolicy),
		trafficPolicies:         make(map[string]*policyv1alpha1.TrafficPolicy),
		routeToGateways:         make(map[string]map[string]struct{}),
		routeToBackends:         make(map[string]map[string]struct{}),
		backendToGateways:       make(map[string]map[string]struct{}),
		certificateToGateways:   make(map[string]map[string]struct{}),
		authPolicyToGateways:    make(map[string]map[string]struct{}),
		trafficPolicyToGateways: make(map[string]map[string]struct{}),
	}
}

func (g *graph) upsertGateway(obj *gatewayv1alpha1.Gateway) {
	if obj == nil {
		return
	}
	g.gateways[objectKeyOf(obj).String()] = obj
	g.rebuildLocked()
}

func (g *graph) deleteGateway(key shared.ObjectKey) {
	delete(g.gateways, key.String())
	g.rebuildLocked()
}

func (g *graph) upsertRoute(obj *gatewayv1alpha1.Route) {
	if obj == nil {
		return
	}
	g.routes[objectKeyOf(obj).String()] = obj
	g.rebuildLocked()
}

func (g *graph) deleteRoute(key shared.ObjectKey) {
	delete(g.routes, key.String())
	g.rebuildLocked()
}

func (g *graph) upsertBackend(obj *gatewayv1alpha1.Backend) {
	if obj == nil {
		return
	}
	g.backends[objectKeyOf(obj).String()] = obj
	g.rebuildLocked()
}

func (g *graph) deleteBackend(key shared.ObjectKey) {
	delete(g.backends, key.String())
	g.rebuildLocked()
}

func (g *graph) upsertCertificate(obj *gatewayv1alpha1.Certificate) {
	if obj == nil {
		return
	}
	g.certificates[objectKeyOf(obj).String()] = obj
	g.rebuildLocked()
}

func (g *graph) deleteCertificate(key shared.ObjectKey) {
	delete(g.certificates, key.String())
	g.rebuildLocked()
}

func (g *graph) upsertAuthPolicy(obj *policyv1alpha1.AuthPolicy) {
	if obj == nil {
		return
	}
	g.authPolicies[objectKeyOf(obj).String()] = obj
	g.rebuildLocked()
}

func (g *graph) deleteAuthPolicy(key shared.ObjectKey) {
	delete(g.authPolicies, key.String())
	g.rebuildLocked()
}

func (g *graph) upsertTrafficPolicy(obj *policyv1alpha1.TrafficPolicy) {
	if obj == nil {
		return
	}
	g.trafficPolicies[objectKeyOf(obj).String()] = obj
	g.rebuildLocked()
}

func (g *graph) deleteTrafficPolicy(key shared.ObjectKey) {
	delete(g.trafficPolicies, key.String())
	g.rebuildLocked()
}

func (g *graph) affectedGatewaysForGateway(key shared.ObjectKey) []shared.ObjectKey {
	if key.Name == "" {
		return nil
	}
	return []shared.ObjectKey{key}
}

func (g *graph) affectedGatewaysForRoute(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.routeToGateways[key.String()])
}

func (g *graph) affectedGatewaysForBackend(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.backendToGateways[key.String()])
}

func (g *graph) affectedGatewaysForCertificate(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.certificateToGateways[key.String()])
}

func (g *graph) affectedGatewaysForAuthPolicy(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.authPolicyToGateways[key.String()])
}

func (g *graph) affectedGatewaysForTrafficPolicy(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.trafficPolicyToGateways[key.String()])
}

func (g *graph) backendsForRoute(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.routeToBackends[key.String()])
}

func (g *graph) gatewaysForBackend(key shared.ObjectKey) []shared.ObjectKey {
	return collectKeys(g.backendToGateways[key.String()])
}

func (g *graph) rebuildLocked() {
	g.routeToGateways = make(map[string]map[string]struct{})
	g.routeToBackends = make(map[string]map[string]struct{})
	g.backendToGateways = make(map[string]map[string]struct{})
	g.certificateToGateways = make(map[string]map[string]struct{})
	g.authPolicyToGateways = make(map[string]map[string]struct{})
	g.trafficPolicyToGateways = make(map[string]map[string]struct{})

	for _, route := range g.routes {
		routeObjectKey := objectKeyOf(route)
		routeKey := routeObjectKey.String()
		gateways := make(map[string]struct{})
		backends := make(map[string]struct{})

		for _, parent := range route.Spec.ParentRefs {
			parentKey := referenceKey(routeObjectKey.Namespace, parent.Name)
			if parentKey.Name == "" {
				continue
			}
			gateways[parentKey.String()] = struct{}{}
		}

		for _, rule := range route.Spec.Rules {
			for _, backendRef := range rule.BackendRefs {
				backendKey := referenceKey(routeObjectKey.Namespace, backendRef.Name)
				if backendKey.Name == "" {
					continue
				}
				backends[backendKey.String()] = struct{}{}
			}
		}

		if len(gateways) != 0 {
			g.routeToGateways[routeKey] = gateways
		}
		if len(backends) != 0 {
			g.routeToBackends[routeKey] = backends
		}
		for backendKey := range backends {
			if g.backendToGateways[backendKey] == nil {
				g.backendToGateways[backendKey] = make(map[string]struct{})
			}
			for gatewayKey := range gateways {
				g.backendToGateways[backendKey][gatewayKey] = struct{}{}
			}
		}
	}

	for _, gateway := range g.gateways {
		gatewayObjectKey := objectKeyOf(gateway)
		gatewayKey := gatewayObjectKey.String()
		for _, listener := range gateway.Spec.Listeners {
			if listener.TLS == nil || listener.TLS.CertificateRef == nil {
				continue
			}
			certificateKey := referenceKey(gatewayObjectKey.Namespace, listener.TLS.CertificateRef.Name)
			if certificateKey.Name == "" {
				continue
			}
			if g.certificateToGateways[certificateKey.String()] == nil {
				g.certificateToGateways[certificateKey.String()] = make(map[string]struct{})
			}
			g.certificateToGateways[certificateKey.String()][gatewayKey] = struct{}{}
		}
	}

	for _, policy := range g.authPolicies {
		policyObjectKey := objectKeyOf(policy)
		policyKey := policyObjectKey.String()
		g.authPolicyToGateways[policyKey] = g.resolvePolicyGateways(policyObjectKey.Namespace, policy.Spec.TargetRefs)
	}

	for _, policy := range g.trafficPolicies {
		policyObjectKey := objectKeyOf(policy)
		policyKey := policyObjectKey.String()
		g.trafficPolicyToGateways[policyKey] = g.resolvePolicyGateways(policyObjectKey.Namespace, policy.Spec.TargetRefs)
	}
}

func (g *graph) resolvePolicyGateways(defaultNamespace string, targetRefs []policyv1alpha1.TargetReference) map[string]struct{} {
	result := make(map[string]struct{})
	for _, target := range targetRefs {
		targetKey := referenceKey(defaultNamespace, target.Name)
		if targetKey.Name == "" {
			continue
		}

		switch {
		case strings.EqualFold(target.Kind, "Gateway"):
			result[targetKey.String()] = struct{}{}
		case strings.EqualFold(target.Kind, "Route"):
			for gatewayKey := range g.routeToGateways[targetKey.String()] {
				result[gatewayKey] = struct{}{}
			}
		case strings.EqualFold(target.Kind, "Backend"):
			for gatewayKey := range g.backendToGateways[targetKey.String()] {
				result[gatewayKey] = struct{}{}
			}
		}
	}
	return result
}

func referenceKey(defaultNamespace, name string) shared.ObjectKey {
	return shared.NewObjectKey(defaultNamespace, name)
}

func objectKeyOf(obj interface {
	GetNamespace() string
	GetName() string
}) shared.ObjectKey {
	return shared.NewObjectKey(obj.GetNamespace(), obj.GetName())
}

func collectKeys(values map[string]struct{}) []shared.ObjectKey {
	if len(values) == 0 {
		return nil
	}

	keys := make([]shared.ObjectKey, 0, len(values))
	for value := range values {
		key, err := shared.ParseObjectKey(value)
		if err != nil {
			continue
		}
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})
	return keys
}
