package index

import (
	"reflect"
	"testing"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
)

func TestObjectKeyStringAndParse(t *testing.T) {
	t.Run("cluster scoped", func(t *testing.T) {
		key := shared.NewObjectKey("", "gateway-a")
		if got, want := key.String(), "gateway-a"; got != want {
			t.Fatalf("unexpected key string: got %q, want %q", got, want)
		}

		parsed, err := shared.ParseObjectKey(key.String())
		if err != nil {
			t.Fatalf("parse key: %v", err)
		}
		if parsed.Namespace != "" || parsed.Name != "gateway-a" {
			t.Fatalf("unexpected parsed key: %#v", parsed)
		}
	})

	t.Run("namespaced", func(t *testing.T) {
		key := shared.NewObjectKey("ingate", "gateway-a")
		if got, want := key.String(), "ingate/gateway-a"; got != want {
			t.Fatalf("unexpected key string: got %q, want %q", got, want)
		}

		parsed, err := shared.ParseObjectKey(key.String())
		if err != nil {
			t.Fatalf("parse key: %v", err)
		}
		if parsed.Namespace != "ingate" || parsed.Name != "gateway-a" {
			t.Fatalf("unexpected parsed key: %#v", parsed)
		}
	})

	t.Run("rejects empty namespace form", func(t *testing.T) {
		if _, err := shared.ParseObjectKey("/gateway-a"); err == nil {
			t.Fatal("expected parse error for empty namespace")
		}
	})
}

func TestAffectedGatewaysForBackendReturnsGatewayKeys(t *testing.T) {
	idx := New()

	idx.UpsertGateway(gateway("public-edge", "cert-a"))
	idx.UpsertRoute(route("catalog-route", []string{"public-edge"}, []string{"catalog-backend"}))
	idx.UpsertBackend(backend("catalog-backend"))

	got := keyStrings(idx.AffectedGatewaysForBackend(shared.NewObjectKey("", "catalog-backend")))
	want := []string{"public-edge"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways (-want +got):\nwant: %v\ngot : %v", want, got)
	}
}

func TestRouteUpdateAndDeleteRefreshResults(t *testing.T) {
	idx := New()

	idx.UpsertGateway(gateway("public-edge", "cert-a"))
	idx.UpsertRoute(route("catalog-route", []string{"public-edge"}, []string{"catalog-backend-a"}))
	idx.UpsertBackend(backend("catalog-backend-a"))
	idx.UpsertBackend(backend("catalog-backend-b"))

	routeKey := shared.NewObjectKey("", "catalog-route")
	backendAKey := shared.NewObjectKey("", "catalog-backend-a")
	backendBKey := shared.NewObjectKey("", "catalog-backend-b")

	if got, want := keyStrings(idx.BackendsForRoute(routeKey)), []string{"catalog-backend-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected backends for route before update: got %v, want %v", got, want)
	}
	if got, want := keyStrings(idx.GatewaysForBackend(backendAKey)), []string{"public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for backend-a before update: got %v, want %v", got, want)
	}
	if got := keyStrings(idx.GatewaysForBackend(backendBKey)); len(got) != 0 {
		t.Fatalf("expected no gateways for backend-b before update, got %v", got)
	}

	idx.UpsertRoute(route("catalog-route", []string{"public-edge"}, []string{"catalog-backend-b"}))

	if got, want := keyStrings(idx.BackendsForRoute(routeKey)), []string{"catalog-backend-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected backends for route after update: got %v, want %v", got, want)
	}
	if got := keyStrings(idx.GatewaysForBackend(backendAKey)); len(got) != 0 {
		t.Fatalf("expected backend-a gateways to refresh after route update, got %v", got)
	}
	if got, want := keyStrings(idx.GatewaysForBackend(backendBKey)), []string{"public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for backend-b after route update: got %v, want %v", got, want)
	}

	idx.DeleteRoute(routeKey)

	if got := keyStrings(idx.BackendsForRoute(routeKey)); len(got) != 0 {
		t.Fatalf("expected route backends to clear after delete, got %v", got)
	}
	if got := keyStrings(idx.GatewaysForBackend(backendBKey)); len(got) != 0 {
		t.Fatalf("expected backend-b gateways to clear after route delete, got %v", got)
	}
}

func TestCertificateAuthPolicyAndTrafficPolicyInfluenceGateways(t *testing.T) {
	idx := New()

	idx.UpsertGateway(gateway("public-edge", "cert-a"))
	idx.UpsertRoute(route("catalog-route", []string{"public-edge"}, []string{"catalog-backend"}))
	idx.UpsertBackend(backend("catalog-backend"))
	idx.UpsertCertificate(certificate("cert-a"))
	idx.UpsertAuthPolicy(authPolicy("auth-a", []policyv1alpha1.TargetReference{{Kind: "Route", Name: "catalog-route"}}))
	idx.UpsertTrafficPolicy(trafficPolicy("traffic-a", []policyv1alpha1.TargetReference{{Kind: "Backend", Name: "catalog-backend"}}))

	if got, want := keyStrings(idx.AffectedGatewaysForCertificate(shared.NewObjectKey("", "cert-a"))), []string{"public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for certificate: got %v, want %v", got, want)
	}
	if got, want := keyStrings(idx.AffectedGatewaysForAuthPolicy(shared.NewObjectKey("", "auth-a"))), []string{"public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for auth policy: got %v, want %v", got, want)
	}
	if got, want := keyStrings(idx.AffectedGatewaysForTrafficPolicy(shared.NewObjectKey("", "traffic-a"))), []string{"public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for traffic policy: got %v, want %v", got, want)
	}
}

func TestNamespacedRelationshipsPreserveNamespace(t *testing.T) {
	idx := New()

	idx.UpsertGateway(namespacedGateway("tenant-a", "public-edge", "cert-a"))
	idx.UpsertRoute(namespacedRoute("tenant-a", "catalog-route", []string{"public-edge"}, []string{"catalog-backend"}))
	idx.UpsertBackend(namespacedBackend("tenant-a", "catalog-backend"))
	idx.UpsertCertificate(namespacedCertificate("tenant-a", "cert-a"))
	idx.UpsertAuthPolicy(namespacedAuthPolicy("tenant-a", "auth-a", []policyv1alpha1.TargetReference{{Kind: "Route", Name: "catalog-route"}}))
	idx.UpsertTrafficPolicy(namespacedTrafficPolicy("tenant-a", "traffic-a", []policyv1alpha1.TargetReference{{Kind: "Backend", Name: "catalog-backend"}}))

	if got, want := keyStrings(idx.AffectedGatewaysForBackend(shared.NewObjectKey("tenant-a", "catalog-backend"))), []string{"tenant-a/public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for namespaced backend: got %v, want %v", got, want)
	}
	if got, want := keyStrings(idx.AffectedGatewaysForCertificate(shared.NewObjectKey("tenant-a", "cert-a"))), []string{"tenant-a/public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for namespaced certificate: got %v, want %v", got, want)
	}
	if got, want := keyStrings(idx.AffectedGatewaysForAuthPolicy(shared.NewObjectKey("tenant-a", "auth-a"))), []string{"tenant-a/public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for namespaced auth policy: got %v, want %v", got, want)
	}
	if got, want := keyStrings(idx.AffectedGatewaysForTrafficPolicy(shared.NewObjectKey("tenant-a", "traffic-a"))), []string{"tenant-a/public-edge"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected gateways for namespaced traffic policy: got %v, want %v", got, want)
	}
}

func gateway(name, cert string) *gatewayv1alpha1.Gateway {
	obj := &gatewayv1alpha1.Gateway{}
	obj.Name = name
	if cert != "" {
		obj.Spec.Listeners = []gatewayv1alpha1.GatewayListener{
			{
				Name: "https",
				TLS: &gatewayv1alpha1.GatewayTLSConfig{
					CertificateRef: &gatewayv1alpha1.LocalObjectReference{Name: cert},
				},
			},
		}
	}
	return obj
}

func route(name string, parents []string, backends []string) *gatewayv1alpha1.Route {
	obj := &gatewayv1alpha1.Route{}
	obj.Name = name
	obj.Spec.ParentRefs = make([]gatewayv1alpha1.ParentReference, 0, len(parents))
	for _, parent := range parents {
		obj.Spec.ParentRefs = append(obj.Spec.ParentRefs, gatewayv1alpha1.ParentReference{Name: parent})
	}
	obj.Spec.Rules = []gatewayv1alpha1.RouteRule{{BackendRefs: make([]gatewayv1alpha1.BackendRef, 0, len(backends))}}
	for _, backend := range backends {
		obj.Spec.Rules[0].BackendRefs = append(obj.Spec.Rules[0].BackendRefs, gatewayv1alpha1.BackendRef{Name: backend, Weight: 1})
	}
	return obj
}

func backend(name string) *gatewayv1alpha1.Backend {
	obj := &gatewayv1alpha1.Backend{}
	obj.Name = name
	return obj
}

func certificate(name string) *gatewayv1alpha1.Certificate {
	obj := &gatewayv1alpha1.Certificate{}
	obj.Name = name
	return obj
}

func authPolicy(name string, refs []policyv1alpha1.TargetReference) *policyv1alpha1.AuthPolicy {
	obj := &policyv1alpha1.AuthPolicy{}
	obj.Name = name
	obj.Spec.TargetRefs = refs
	return obj
}

func trafficPolicy(name string, refs []policyv1alpha1.TargetReference) *policyv1alpha1.TrafficPolicy {
	obj := &policyv1alpha1.TrafficPolicy{}
	obj.Name = name
	obj.Spec.TargetRefs = refs
	return obj
}

func namespacedGateway(namespace, name, cert string) *gatewayv1alpha1.Gateway {
	obj := gateway(name, cert)
	obj.Namespace = namespace
	return obj
}

func namespacedRoute(namespace, name string, parents []string, backends []string) *gatewayv1alpha1.Route {
	obj := route(name, parents, backends)
	obj.Namespace = namespace
	return obj
}

func namespacedBackend(namespace, name string) *gatewayv1alpha1.Backend {
	obj := backend(name)
	obj.Namespace = namespace
	return obj
}

func namespacedCertificate(namespace, name string) *gatewayv1alpha1.Certificate {
	obj := certificate(name)
	obj.Namespace = namespace
	return obj
}

func namespacedAuthPolicy(namespace, name string, refs []policyv1alpha1.TargetReference) *policyv1alpha1.AuthPolicy {
	obj := authPolicy(name, refs)
	obj.Namespace = namespace
	return obj
}

func namespacedTrafficPolicy(namespace, name string, refs []policyv1alpha1.TargetReference) *policyv1alpha1.TrafficPolicy {
	obj := trafficPolicy(name, refs)
	obj.Namespace = namespace
	return obj
}

func keyStrings(keys []shared.ObjectKey) []string {
	if len(keys) == 0 {
		return nil
	}
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, key.String())
	}
	return items
}
