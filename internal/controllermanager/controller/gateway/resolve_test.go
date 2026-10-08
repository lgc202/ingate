package gateway

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type resolutionView struct {
	Gateways         []string
	Routes           []string
	Upstreams        []string
	GatewayConflicts []string
	RouteRejections  []string
}

func TestResolveGatewayConflict(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("later", 2,
			listener("other", 9090, ""),
			listener("conflicting", 8080, ""),
		),
		gatewayResource("first", 1, listener("http", 8080, "api.example.com")),
		gatewayResource("available", 3, listener("http", 9090, "")),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("rejected", 1, "later", "other", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("accepted", 2, "available", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
	}
	upstreams := []gatewayv1alpha1.Upstream{upstreamResource("backend")}

	got := viewOf(resolve(gateways, routes, upstreams))
	want := resolutionView{
		Gateways:         []string{"first", "available"},
		Routes:           []string{"accepted"},
		Upstreams:        []string{"backend"},
		GatewayConflicts: []string{"later/conflicting: first/http"},
		RouteRejections:  []string{"rejected: GatewayRejected (later)"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resolve(gateways with overlapping listeners) mismatch (-want +got):\n%s", diff)
	}
	if gateways[0].Name != "later" {
		t.Errorf("resolve() changed input gateway order: first = %q, want %q", gateways[0].Name, "later")
	}
}

func TestResolveDifferentHostsSharePort(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("second", 2, listener("http", 8080, "b.example.com")),
		gatewayResource("first", 1, listener("http", 8080, "a.example.com")),
	}

	got := viewOf(resolve(gateways, nil, nil))
	want := resolutionView{Gateways: []string{"first", "second"}}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resolve(gateways with distinct hosts on one port) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveRouteReferences(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("edge", 1, listener("http", 8080, "")),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("missing-gateway", 1, "unknown", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("missing-listener", 2, "edge", "other", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("missing-upstream", 3, "edge", "http", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("valid", 4, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
	}

	got := viewOf(resolve(gateways, routes, []gatewayv1alpha1.Upstream{upstreamResource("backend")}))
	want := resolutionView{
		Gateways:  []string{"edge"},
		Routes:    []string{"valid"},
		Upstreams: []string{"backend"},
		RouteRejections: []string{
			"missing-gateway: GatewayMissing (unknown)",
			"missing-listener: ListenerMissing (other)",
			"missing-upstream: UpstreamMissing (unknown)",
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resolve(routes with missing references) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveRouteMatchConflict(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("edge", 1, listener("http", 8080, ""), listener("other", 9090, "")),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("later", 2, "edge", "http", "unused", gatewayv1alpha1.PathMatchPrefix),
		routeResource("first", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("exact", 3, "edge", "http", "backend", gatewayv1alpha1.PathMatchExact),
		routeResource("other-listener", 4, "edge", "other", "backend", gatewayv1alpha1.PathMatchPrefix),
	}
	upstreams := []gatewayv1alpha1.Upstream{upstreamResource("backend"), upstreamResource("unused")}

	got := viewOf(resolve(gateways, routes, upstreams))
	want := resolutionView{
		Gateways:        []string{"edge"},
		Routes:          []string{"first", "exact", "other-listener"},
		Upstreams:       []string{"backend"},
		RouteRejections: []string{"later: MatchConflict (first)"},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resolve(routes with equal and distinct matches) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolvePathPrefixTrailingSlashConflict(t *testing.T) {
	first := routeResource("first", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	withSlash := routeResource("with-slash", 2, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	withSlash.Spec.Path.Value = "/api/"
	exact := routeResource("exact", 3, "edge", "http", "backend", gatewayv1alpha1.PathMatchExact)
	exact.Spec.Path.Value = "/api/"

	result := resolve(
		[]gatewayv1alpha1.Gateway{gatewayResource("edge", 1, listener("http", 8080, ""))},
		[]gatewayv1alpha1.Route{withSlash, exact, first},
		[]gatewayv1alpha1.Upstream{upstreamResource("backend")},
	)
	want := resolutionView{
		Gateways:        []string{"edge"},
		Routes:          []string{"first", "exact"},
		Upstreams:       []string{"backend"},
		RouteRejections: []string{"with-slash: MatchConflict (first)"},
	}

	if diff := cmp.Diff(want, viewOf(result)); diff != "" {
		t.Errorf("resolve(PathPrefix /api and /api/, Exact /api/) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveOrderAndRecovery(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("z", 2, listener("http", 8080, "")),
		gatewayResource("b", 1, listener("http", 8080, "")),
		gatewayResource("a", 1, listener("http", 8080, "")),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("z", 2, "a", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("b", 1, "a", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("a", 1, "a", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
	}
	upstreams := []gatewayv1alpha1.Upstream{upstreamResource("backend")}

	before := viewOf(resolve(gateways, routes, nil))
	wantBefore := resolutionView{
		Gateways:         []string{"a"},
		GatewayConflicts: []string{"b/http: a/http", "z/http: a/http"},
		RouteRejections: []string{
			"a: UpstreamMissing (backend)",
			"b: UpstreamMissing (backend)",
			"z: UpstreamMissing (backend)",
		},
	}

	if diff := cmp.Diff(wantBefore, before); diff != "" {
		t.Errorf("resolve(resources before Upstream exists) mismatch (-want +got):\n%s", diff)
	}

	want := resolutionView{
		Gateways:         []string{"a"},
		Routes:           []string{"a"},
		Upstreams:        []string{"backend"},
		GatewayConflicts: []string{"b/http: a/http", "z/http: a/http"},
		RouteRejections:  []string{"b: MatchConflict (a)", "z: MatchConflict (a)"},
	}

	if diff := cmp.Diff(want, viewOf(resolve(gateways, routes, upstreams))); diff != "" {
		t.Errorf("resolve(resources after Upstream appears) mismatch (-want +got):\n%s", diff)
	}

	got := viewOf(resolve(
		[]gatewayv1alpha1.Gateway{gateways[2], gateways[0], gateways[1]},
		[]gatewayv1alpha1.Route{routes[2], routes[0], routes[1]},
		upstreams,
	))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resolve(reordered resources) mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"a"}, got.Gateways); diff != "" {
		t.Errorf("resolve(reordered gateways) winner mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"a"}, got.Routes); diff != "" {
		t.Errorf("resolve(reordered routes) winner mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(resolutionView{}, viewOf(resolve(nil, nil, nil))); diff != "" {
		t.Errorf("resolve(empty resources) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveReferenceRejectionPrecedesMatchConflict(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("edge", 1, listener("http", 8080, "")),
		gatewayResource("blocked", 2, listener("http", 8080, "")),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("missing-gateway", 1, "unknown", "other", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("rejected-gateway", 2, "blocked", "other", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("missing-listener", 3, "edge", "other", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("missing-upstream", 4, "edge", "http", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("accepted", 5, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("missing-upstream-after-owner", 6, "edge", "http", "unknown", gatewayv1alpha1.PathMatchPrefix),
		routeResource("conflict", 7, "edge", "http", "unused", gatewayv1alpha1.PathMatchPrefix),
	}
	upstreams := []gatewayv1alpha1.Upstream{upstreamResource("unused"), upstreamResource("backend")}
	want := resolutionView{
		Gateways:         []string{"edge"},
		Routes:           []string{"accepted"},
		Upstreams:        []string{"backend"},
		GatewayConflicts: []string{"blocked/http: edge/http"},
		RouteRejections: []string{
			"missing-gateway: GatewayMissing (unknown)",
			"rejected-gateway: GatewayRejected (blocked)",
			"missing-listener: ListenerMissing (other)",
			"missing-upstream: UpstreamMissing (unknown)",
			"missing-upstream-after-owner: UpstreamMissing (unknown)",
			"conflict: MatchConflict (accepted)",
		},
	}

	if diff := cmp.Diff(want, viewOf(resolve(gateways, routes, upstreams))); diff != "" {
		t.Errorf("resolve(routes with multiple rejection causes) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveDoesNotModifyInputs(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{
		gatewayResource("later", 2, listener("http", 8080, "")),
		gatewayResource("edge", 1, listener("http", 8080, "")),
	}
	routes := []gatewayv1alpha1.Route{
		routeResource("later", 2, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
		routeResource("first", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix),
	}
	routes[0].Spec.Path.Value = "/api/"
	upstreams := []gatewayv1alpha1.Upstream{upstreamResource("unused"), upstreamResource("backend")}
	wantGateways := (&gatewayv1alpha1.GatewayList{Items: gateways}).DeepCopy().Items
	wantRoutes := (&gatewayv1alpha1.RouteList{Items: routes}).DeepCopy().Items
	wantUpstreams := (&gatewayv1alpha1.UpstreamList{Items: upstreams}).DeepCopy().Items

	resolve(gateways, routes, upstreams)
	if diff := cmp.Diff(wantGateways, gateways); diff != "" {
		t.Errorf("resolve(unsorted gateways) modified input (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantRoutes, routes); diff != "" {
		t.Errorf("resolve(routes with canonicalizable paths) modified input (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantUpstreams, upstreams); diff != "" {
		t.Errorf("resolve(upstreams with unused entries) modified input (-want +got):\n%s", diff)
	}
}

func viewOf(result resolution) resolutionView {
	var view resolutionView
	for _, gateway := range result.gateways {
		view.Gateways = append(view.Gateways, gateway.Name)
	}
	for _, route := range result.routes {
		view.Routes = append(view.Routes, route.Name)
	}
	for _, upstream := range result.upstreams {
		view.Upstreams = append(view.Upstreams, upstream.Name)
	}
	for _, conflict := range result.gatewayConflicts {
		view.GatewayConflicts = append(view.GatewayConflicts, fmt.Sprintf(
			"%s/%s: %s/%s",
			conflict.gateway.Name, conflict.listenerName,
			conflict.owner.Name, conflict.ownerListenerName,
		))
	}
	for _, rejection := range result.routeRejections {
		view.RouteRejections = append(view.RouteRejections, fmt.Sprintf(
			"%s: %s (%s)", rejection.route.Name, rejection.reason, rejection.relatedName,
		))
	}

	return view
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
	upstreamName string,
	pathType gatewayv1alpha1.PathMatchType,
) gatewayv1alpha1.Route {
	return gatewayv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(time.Unix(created, 0))},
		Spec: gatewayv1alpha1.RouteSpec{
			GatewayName: gatewayName, ListenerName: listenerName, UpstreamName: upstreamName,
			Path: gatewayv1alpha1.PathMatch{Type: pathType, Value: "/api"},
		},
	}
}

func upstreamResource(name string) gatewayv1alpha1.Upstream {
	return gatewayv1alpha1.Upstream{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: gatewayv1alpha1.UpstreamSpec{Endpoints: []gatewayv1alpha1.Endpoint{
			{Address: "127.0.0.1", Port: 8081, Weight: 1},
		}},
	}
}
