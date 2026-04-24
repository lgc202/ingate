package gatewaycompiler

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

func TestBuildLogicalGatewayMergesRouteBackendAndPolicies(t *testing.T) {
	bundle := testResourceBundle()

	logical, err := BuildLogicalGateway(bundle)
	if err != nil {
		t.Fatalf("BuildLogicalGateway() error = %v", err)
	}
	if logical.Meta.Name != "public-edge" {
		t.Fatalf("unexpected logical gateway name: %q", logical.Meta.Name)
	}
	if len(logical.Routes) != 1 {
		t.Fatalf("expected 1 logical route, got %d", len(logical.Routes))
	}
	if len(logical.Backends) != 1 {
		t.Fatalf("expected 1 logical backend, got %d", len(logical.Backends))
	}
	if logical.Trace == nil || len(logical.Trace.Sources) == 0 {
		t.Fatalf("expected trace sources, got %#v", logical.Trace)
	}

	if len(logical.Listeners) != 1 {
		t.Fatalf("expected 1 logical listener, got %d", len(logical.Listeners))
	}
	if got := logical.Policies.GatewayAuth; got == nil || len(got.Policies) != 1 {
		t.Fatalf("expected gateway auth summary, got %#v", got)
	}
	if got := logical.Routes[0].AuthSummary; got == nil || len(got.Policies) != 1 {
		t.Fatalf("expected route auth summary, got %#v", got)
	}
	if got := logical.Backends[0].TrafficSummary; got == nil || len(got.Policies) != 1 {
		t.Fatalf("expected backend traffic summary, got %#v", got)
	}
	if got := logical.Listeners[0].TLS; got == nil || got.CertificateName != "cert-a" || got.SecretName != "secret-a" {
		t.Fatalf("expected listener tls refs, got %#v", got)
	}
}

func testResourceBundle() *ResourceBundle {
	return &ResourceBundle{
		Gateway: gateway("public-edge"),
		Routes: []*gatewayv1alpha1.Route{
			route("catalog-route", "public-edge", "catalog-backend"),
		},
		Backends: []*gatewayv1alpha1.Backend{
			backend("catalog-backend"),
		},
		Certificates: []*gatewayv1alpha1.Certificate{
			certificate("cert-a"),
		},
		GatewayAuthPolicies: []*policyv1alpha1.AuthPolicy{
			authPolicy("gateway-auth", "Gateway", "public-edge"),
		},
		RouteAuthPolicies: map[string][]*policyv1alpha1.AuthPolicy{
			"catalog-route": {
				authPolicy("route-auth", "Route", "catalog-route"),
			},
		},
		BackendTrafficPolicies: map[string][]*policyv1alpha1.TrafficPolicy{
			"catalog-backend": {
				trafficPolicy("backend-traffic", "Backend", "catalog-backend"),
			},
		},
	}
}

func gateway(name string) *gatewayv1alpha1.Gateway {
	return &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1alpha1.GatewaySpec{
			Listeners: []gatewayv1alpha1.GatewayListener{
				{
					Name:      "https",
					Protocol:  "HTTPS",
					Port:      443,
					Hostname:  "example.com",
					Hostnames: []string{"example.com", "*.example.com"},
					TLS: &gatewayv1alpha1.GatewayTLSConfig{
						CertificateRef: &gatewayv1alpha1.LocalObjectReference{Name: "cert-a"},
					},
				},
			},
		},
	}
}

func route(name, parent, backendName string) *gatewayv1alpha1.Route {
	return &gatewayv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1alpha1.RouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{{Name: parent}},
			Hostnames:  []string{"example.com"},
			Rules: []gatewayv1alpha1.RouteRule{
				{
					Matches: []gatewayv1alpha1.HTTPRouteMatch{
						{
							Method:  "GET",
							Path:    &gatewayv1alpha1.HTTPPathMatch{Type: "PathPrefix", Value: "/catalog"},
							Headers: []gatewayv1alpha1.HTTPHeaderMatch{{Name: "x-env", Value: "prod"}},
						},
					},
					BackendRefs: []gatewayv1alpha1.BackendRef{{Name: backendName, Port: 8080, Weight: 100}},
					Filters: []gatewayv1alpha1.HTTPRouteFilter{
						{
							URLRewrite: &gatewayv1alpha1.HTTPURLRewriteFilter{
								Path: &gatewayv1alpha1.HTTPPathModifier{Type: "ReplacePrefixMatch", ReplacePrefixMatch: "/v1"},
							},
							RequestHeaderModifier: &gatewayv1alpha1.HTTPHeaderFilter{
								Set: []gatewayv1alpha1.HTTPHeader{{Name: "x-added", Value: "true"}},
							},
						},
					},
				},
			},
		},
	}
}

func backend(name string) *gatewayv1alpha1.Backend {
	return &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1alpha1.BackendSpec{
			Protocol:    gatewayv1alpha1.BackendProtocolHTTP,
			LoadBalance: &gatewayv1alpha1.LoadBalanceSpec{Policy: "RoundRobin"},
			DefaultPort: 8080,
			Static:      &gatewayv1alpha1.StaticBackendSpec{Endpoints: []gatewayv1alpha1.BackendEndpoint{{Address: "10.0.0.1", Port: 8080, Weight: 1, Healthy: true}}},
		},
	}
}

func certificate(name string) *gatewayv1alpha1.Certificate {
	return &gatewayv1alpha1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gatewayv1alpha1.CertificateSpec{
			SecretRef: gatewayv1alpha1.LocalObjectReference{Name: "secret-a"},
			Domains:   []string{"example.com"},
		},
	}
}

func authPolicy(name, kind, target string) *policyv1alpha1.AuthPolicy {
	return &policyv1alpha1.AuthPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: policyv1alpha1.AuthPolicySpec{
			TargetRefs: []policyv1alpha1.TargetReference{{Kind: kind, Name: target}},
			Type:       "JWT",
			JWT:        &policyv1alpha1.JWTAuthSpec{Issuer: "issuer-a"},
		},
	}
}

func trafficPolicy(name, kind, target string) *policyv1alpha1.TrafficPolicy {
	return &policyv1alpha1.TrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: policyv1alpha1.TrafficPolicySpec{
			TargetRefs: []policyv1alpha1.TargetReference{{Kind: kind, Name: target}},
			Timeout:    &policyv1alpha1.TimeoutSpec{Duration: "5s"},
		},
	}
}
