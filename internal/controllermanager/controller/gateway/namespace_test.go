package gateway

import (
	"context"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCompileSeparatesNamespacedResources(t *testing.T) {
	snapshot := namespacedHTTPResources()
	before := snapshot
	before.gateways = *snapshot.gateways.DeepCopy()
	before.httpRoutes = *snapshot.httpRoutes.DeepCopy()
	before.backends = *snapshot.backends.DeepCopy()
	got := compile(resolve(snapshot.gateways.Items, snapshot.httpRoutes.Items, snapshot.backends.Items))
	want := dataplane.Config{Listeners: []dataplane.Listener{{Port: 8080}}}
	for _, namespace := range []string{"team-a", "team-b"} {
		want.Listeners[0].Hosts = append(want.Listeners[0].Hosts, dataplane.Host{
			Name:     "gateway/" + namespace + "/edge/listener/http",
			Hostname: namespace + ".example.com",
			Routes: []dataplane.Route{{
				Name:        namespace + "/api/rule/0/match/0",
				PathType:    dataplane.PathMatchPrefix,
				Path:        "/api",
				BackendName: namespace + "/backend",
			}},
		})
		want.Backends = append(want.Backends, dataplane.Backend{
			Name:      namespace + "/backend",
			Endpoints: []dataplane.Endpoint{{Address: namespace + ".backend.example.com", Port: 8081, Weight: 1}},
		})
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("compile(same names in two namespaces) mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(before, snapshot, cmp.AllowUnexported(resources{})); diff != "" {
		t.Errorf("compile(namespaced resources) changed snapshot (-want +got):\n%s", diff)
	}

	slices.Reverse(snapshot.gateways.Items)
	slices.Reverse(snapshot.httpRoutes.Items)
	slices.Reverse(snapshot.backends.Items)
	reversed := compile(resolve(snapshot.gateways.Items, snapshot.httpRoutes.Items, snapshot.backends.Items))
	if diff := cmp.Diff(want, reversed); diff != "" {
		t.Errorf("compile(reversed namespaced resources) mismatch (-want +got):\n%s", diff)
	}
}

func TestResolveDoesNotAttachToAnotherNamespace(t *testing.T) {
	snapshot := namespacedHTTPResources()
	result := resolve(snapshot.gateways.Items[1:], snapshot.httpRoutes.Items[:1], snapshot.backends.Items)
	if len(result.bindings) != 0 || len(result.backends) != 0 {
		t.Errorf("resolve(team-a route, team-b Gateway) bindings = %+v, backends = %+v, want none", result.bindings, result.backends)
	}
	if len(result.parents) != 1 {
		t.Fatalf("resolve(team-a route, team-b Gateway) parents = %d, want 1", len(result.parents))
	}
	if got := result.parents[0]; got.accepted || got.reason != "NoMatchingParent" || got.missingBackend != "" {
		t.Errorf("resolve(team-a route, team-b Gateway) parent = %+v, want rejected parent with resolved local Backend", got)
	}
}

func TestResolveDoesNotForwardToAnotherNamespace(t *testing.T) {
	snapshot := namespacedHTTPResources()
	result := resolve(snapshot.gateways.Items[:1], snapshot.httpRoutes.Items[:1], snapshot.backends.Items[1:])
	config := compile(result)
	want := dataplane.Config{Listeners: []dataplane.Listener{{
		Port: 8080,
		Hosts: []dataplane.Host{{
			Name:     "gateway/team-a/edge/listener/http",
			Hostname: "team-a.example.com",
			Routes:   []dataplane.Route{{Name: "team-a/api/rule/0/match/0", PathType: dataplane.PathMatchPrefix, Path: "/api"}},
		}},
	}}}
	if diff := cmp.Diff(want, config); diff != "" {
		t.Errorf("compile(team-a route, only team-b Backend) mismatch (-want +got):\n%s", diff)
	}
	if len(result.parents) != 1 {
		t.Fatalf("resolve(team-a route, only team-b Backend) parents = %d, want 1", len(result.parents))
	}
	if got := result.parents[0]; !got.accepted || got.missingBackend != "backend" {
		t.Errorf("resolve(team-a route, only team-b Backend) parent = %+v, want accepted and missing local backend", got)
	}
}

func TestReconcileSeparatesNamespacedRouteStatuses(t *testing.T) {
	snapshot := namespacedHTTPResources()
	apiClient := newFakeClient(t,
		&snapshot.gateways.Items[0], &snapshot.gateways.Items[1],
		&snapshot.httpRoutes.Items[0], &snapshot.httpRoutes.Items[1], &snapshot.backends.Items[0],
	)
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))
	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(namespaced resources) error = %v", err)
	}

	var before []gatewayv1alpha1.HTTPRoute
	for _, namespace := range []string{"team-a", "team-b"} {
		key := client.ObjectKey{Namespace: namespace, Name: "api"}
		var route gatewayv1alpha1.HTTPRoute
		if err := apiClient.Get(t.Context(), key, &route); err != nil {
			t.Fatalf("Get(HTTPRoute %s) error = %v", key, err)
		}
		before = append(before, route)
		if len(route.Status.Parents) != 1 {
			t.Fatalf("HTTPRoute %s parents = %d, want 1", key, len(route.Status.Parents))
		}
		assertAcceptedCondition(t, route.Status.Parents[0].Conditions, metav1.ConditionTrue, "Accepted", route.Generation)
		status, reason := metav1.ConditionTrue, "ResolvedRefs"
		if namespace == "team-b" {
			status, reason = metav1.ConditionFalse, "BackendNotFound"
		}
		condition := meta.FindStatusCondition(route.Status.Parents[0].Conditions, "ResolvedRefs")
		if condition == nil || condition.Status != status || condition.Reason != reason {
			t.Errorf("HTTPRoute %s ResolvedRefs = %+v, want %s/%s", key, condition, status, reason)
		}
	}

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(unchanged namespaced resources) error = %v", err)
	}
	for _, previous := range before {
		key := client.ObjectKeyFromObject(&previous)
		var current gatewayv1alpha1.HTTPRoute
		if err := apiClient.Get(t.Context(), key, &current); err != nil {
			t.Fatalf("Get(HTTPRoute %s after repeated reconcile) error = %v", key, err)
		}
		if diff := cmp.Diff(previous, current); diff != "" {
			t.Errorf("Reconcile(unchanged HTTPRoute %s) wrote object (-want +got):\n%s", key, diff)
		}
	}
}

func TestReconcileListenerConflictsAcrossNamespaces(t *testing.T) {
	snapshot := namespacedHTTPResources()
	for i := range snapshot.gateways.Items {
		snapshot.gateways.Items[i].Spec.Listeners[0].Hostname = ""
	}
	apiClient := newFakeClient(t, &snapshot.gateways.Items[0], &snapshot.gateways.Items[1])
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))
	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(conflicting listeners in different namespaces) error = %v", err)
	}
	for _, namespace := range []string{"team-a", "team-b"} {
		key := client.ObjectKey{Namespace: namespace, Name: "edge"}
		var gateway gatewayv1alpha1.Gateway
		if err := apiClient.Get(t.Context(), key, &gateway); err != nil {
			t.Fatalf("Get(Gateway %s) error = %v", key, err)
		}
		status, reason := metav1.ConditionTrue, "Accepted"
		if namespace == "team-b" {
			status, reason = metav1.ConditionFalse, "ListenerConflict"
		}
		assertAcceptedCondition(t, gateway.Status.Conditions, status, reason, gateway.Generation)
	}
}

func namespacedHTTPResources() resources {
	var snapshot resources
	for _, namespace := range []string{"team-a", "team-b"} {
		gateway := gatewayResource("edge", 1, listener("http", 8080, namespace+".example.com"))
		gateway.Namespace = namespace
		route := routeResource("api", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
		route.Namespace = namespace
		backend := backendResource("backend")
		backend.Namespace = namespace
		backend.Spec.Endpoints[0].Address = namespace + ".backend.example.com"
		snapshot.gateways.Items = append(snapshot.gateways.Items, gateway)
		snapshot.httpRoutes.Items = append(snapshot.httpRoutes.Items, route)
		snapshot.backends.Items = append(snapshot.backends.Items, backend)
	}

	return snapshot
}
