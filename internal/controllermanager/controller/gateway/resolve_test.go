package gateway

import (
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestResolveGatewayConflictAndHostSharing(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("later", 2, listener("free", 9090, ""), listener("conflict", 8080, "")),
		gatewayResource("first", 1, listener("http", 8080, "a.example.com")),
		gatewayResource("shared", 3, listener("http", 8080, "b.example.com")),
		gatewayResource("available", 4, listener("http", 9090, "")),
	}
	before := (&gatewayv1alpha1.GatewayList{Items: gateways}).DeepCopy().Items
	result := resolve(gateways, nil, nil)
	var names []string
	for _, gateway := range result.gateways {
		names = append(names, gateway.Name)
	}
	if diff := cmp.Diff([]string{"first", "shared", "available"}, names); diff != "" {
		t.Errorf("accepted gateways (-want +got):\n%s", diff)
	}
	if len(result.gatewayConflicts) != 1 || result.gatewayConflicts[0].gateway.Name != "later" {
		t.Errorf("conflicts = %+v; want whole later Gateway rejected", result.gatewayConflicts)
	}
	if diff := cmp.Diff(before, gateways); diff != "" {
		t.Errorf("input mutated:\n%s", diff)
	}
}

func TestResolveParentsAndMissingBackend(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{gatewayResource("edge", 1, listener("a", 8080, ""), listener("b", 9090, ""))}
	route := routeResource("public", 1, "edge", "", "missing", gatewayv1alpha1.PathMatchPrefix)
	route.Spec.ParentRefs = append(route.Spec.ParentRefs, gatewayv1alpha1.ParentReference{Name: "edge", SectionName: "absent"}, gatewayv1alpha1.ParentReference{Name: "unknown"})
	result := resolve(gateways, []gatewayv1alpha1.HTTPRoute{route}, nil)
	if len(result.bindings) != 2 {
		t.Fatalf("bindings = %d, want both HTTP listeners", len(result.bindings))
	}
	if len(result.parents) != 3 {
		t.Fatalf("parents = %d, want 3", len(result.parents))
	}
	for i, parent := range result.parents {
		if parent.accepted != (i == 0) {
			t.Errorf("parent %d accepted = %v", i, parent.accepted)
		}
		if i > 0 && parent.reason != "NoMatchingParent" {
			t.Errorf("unmatched parent reason = %q", parent.reason)
		}
		if parent.missingBackend != "missing" {
			t.Errorf("missing backend = %q", parent.missingBackend)
		}
	}
	for _, binding := range result.bindings {
		if binding.backendName != "" {
			t.Errorf("missing target forwarded to %q", binding.backendName)
		}
	}
	if len(result.backends) != 0 {
		t.Errorf("compiled missing backend: %+v", result.backends)
	}
}

func TestResolveKeepsOverlapsAndStableRuleOrder(t *testing.T) {
	gateway := gatewayResource("edge", 1, listener("http", 8080, ""))
	first := routeResource("z", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	first.Spec.Rules[0].Name = "z-rule"
	secondRule := *first.Spec.Rules[0].DeepCopy()
	secondRule.Name = "a-rule"
	first.Spec.Rules = append(first.Spec.Rules, secondRule)
	second := routeResource("a-later", 2, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	tie := routeResource("a", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	routes := []gatewayv1alpha1.HTTPRoute{second, first, tie}
	before := (&gatewayv1alpha1.HTTPRouteList{Items: routes}).DeepCopy().Items
	result := resolve([]gatewayv1alpha1.Gateway{gateway}, routes, []gatewayv1alpha1.Backend{backendResource("backend"), backendResource("unused")})
	var names []string
	var indexes []int
	for _, binding := range result.bindings {
		names = append(names, binding.route.Name)
		indexes = append(indexes, binding.ruleIndex)
	}
	if !slices.Equal(names, []string{"a", "z", "z", "a-later"}) || !slices.Equal(indexes, []int{0, 0, 1, 0}) {
		t.Errorf("order = %v %v; want age, name, rule list order with overlaps kept", names, indexes)
	}
	if len(result.backends) != 1 || result.backends[0].Name != "backend" {
		t.Errorf("backends = %+v, want only used target", result.backends)
	}
	originalConfig := compile(result)
	slices.Reverse(routes)
	reversed := resolve([]gatewayv1alpha1.Gateway{gateway}, routes, []gatewayv1alpha1.Backend{backendResource("backend")})
	if diff := cmp.Diff(originalConfig, compile(reversed)); diff != "" {
		t.Errorf("permutation changed config:\n%s", diff)
	}
	slices.Reverse(routes)
	if diff := cmp.Diff(before, routes); diff != "" {
		t.Errorf("input route mutated:\n%s", diff)
	}
}

func TestResolveDefaultsAndORMatches(t *testing.T) {
	route := routeResource("public", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	route.Spec.Rules[0].Matches = []gatewayv1alpha1.HTTPRouteMatch{{}, {Path: &gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchExact, Value: "/api/"}}}
	result := resolve(
		[]gatewayv1alpha1.Gateway{gatewayResource("edge", 1, listener("http", 8080, ""))},
		[]gatewayv1alpha1.HTTPRoute{route},
		[]gatewayv1alpha1.Backend{backendResource("backend")},
	)
	if len(result.bindings) != 2 || result.bindings[0].path.Value != "/" || result.bindings[1].path.Value != "/api/" {
		t.Fatalf("OR/default matches = %+v", result.bindings)
	}
	for _, tt := range []struct {
		path *gatewayv1alpha1.PathMatch
		want gatewayv1alpha1.PathMatch
	}{
		{want: gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchPrefix, Value: "/"}},
		{path: &gatewayv1alpha1.PathMatch{Value: "/api/"}, want: gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchPrefix, Value: "/api"}},
		{path: &gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchExact, Value: "/api/"}, want: gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchExact, Value: "/api/"}},
	} {
		if got := defaultPath(tt.path); got != tt.want {
			t.Errorf("defaultPath(%+v) = %+v, want %+v", tt.path, got, tt.want)
		}
	}
}

func gatewayResource(name string, created int64, listeners ...gatewayv1alpha1.Listener) gatewayv1alpha1.Gateway {
	return gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(time.Unix(created, 0))},
		Spec:       gatewayv1alpha1.GatewaySpec{Listeners: listeners},
	}
}

func listener(name string, port int32, hostname string) gatewayv1alpha1.Listener {
	return gatewayv1alpha1.Listener{Name: name, Protocol: gatewayv1alpha1.ProtocolHTTP, Port: port, Hostname: hostname}
}

func routeResource(
	name string,
	created int64,
	gatewayName string,
	listenerName string,
	backendName string,
	pathType gatewayv1alpha1.PathMatchType,
) gatewayv1alpha1.HTTPRoute {
	return gatewayv1alpha1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Unix(created, 0)),
		},
		Spec: gatewayv1alpha1.HTTPRouteSpec{
			ParentRefs: []gatewayv1alpha1.ParentReference{{Name: gatewayName, SectionName: listenerName}},
			Rules: []gatewayv1alpha1.HTTPRouteRule{{
				Matches:     []gatewayv1alpha1.HTTPRouteMatch{{Path: &gatewayv1alpha1.PathMatch{Type: pathType, Value: "/api"}}},
				BackendRefs: []gatewayv1alpha1.BackendReference{{Name: backendName}},
			}},
		},
	}
}

func backendResource(name string) gatewayv1alpha1.Backend {
	return gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.BackendSpec{Endpoints: []gatewayv1alpha1.Endpoint{
			{Address: "127.0.0.1", Port: 8081, Weight: 1},
		}},
	}
}
