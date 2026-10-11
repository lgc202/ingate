package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestReconcileWritesAcceptedStatuses(t *testing.T) {
	first := gatewayResource("first", 1, listener("http", 8080, ""))
	conflicting := gatewayResource("conflicting", 2, listener("http", 8080, ""))
	valid := routeResource("valid", 1, "first", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	rejected := routeResource("rejected", 2, "conflicting", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	first.Generation = 3
	conflicting.Generation = 4
	valid.Generation = 5
	rejected.Generation = 6
	backend := backendResource("backend")
	apiClient := newFakeClient(t, &first, &conflicting, &valid, &rejected, &backend)
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	assertGatewayCondition(t, apiClient, "first", metav1.ConditionTrue, "Accepted", 3)
	assertGatewayCondition(t, apiClient, "conflicting", metav1.ConditionFalse, "ListenerConflict", 4)
	assertHTTPRouteCondition(t, apiClient, "valid", metav1.ConditionTrue, "Accepted", 5)
	assertHTTPRouteCondition(t, apiClient, "rejected", metav1.ConditionFalse, "NoMatchingParent", 6)

	var stored gatewayv1alpha1.Backend
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: "backend"}, &stored); err != nil {
		t.Fatalf("Get(Backend) error = %v", err)
	}
	if len(stored.Status.Conditions) != 0 {
		t.Errorf("Backend conditions = %v, want none", stored.Status.Conditions)
	}
}

func TestReconcileUpdatesStatusOnlyWhenAcceptanceChanges(t *testing.T) {
	gateway := gatewayResource("gateway", 1, listener("http", 8080, ""))
	route := routeResource("route", 1, "gateway", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	gateway.Generation = 2
	route.Generation = 7
	var updates int
	apiClient := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(&gateway, &route).
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.HTTPRoute{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(
				ctx context.Context,
				writer client.Client,
				name string,
				object client.Object,
				options ...client.SubResourceUpdateOption,
			) error {
				updates++
				return writer.SubResource(name).Update(ctx, object, options...)
			},
		}).
		Build()
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))

	for range 2 {
		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(missing Backend) error = %v", err)
		}
	}
	if updates != 2 {
		t.Errorf("status updates before Backend exists = %d, want 2", updates)
	}
	assertHTTPRouteCondition(t, apiClient, "route", metav1.ConditionTrue, "Accepted", 7)
	assertHTTPRouteResolvedRefs(t, apiClient, "route", metav1.ConditionFalse, "BackendNotFound")

	backend := backendResource("backend")
	if err := apiClient.Create(t.Context(), &backend); err != nil {
		t.Fatalf("Create(Backend) error = %v", err)
	}
	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(available Backend) error = %v", err)
	}
	if updates != 3 {
		t.Errorf("status updates after Backend appears = %d, want 3", updates)
	}
	assertHTTPRouteCondition(t, apiClient, "route", metav1.ConditionTrue, "Accepted", 7)
	assertHTTPRouteResolvedRefs(t, apiClient, "route", metav1.ConditionTrue, "ResolvedRefs")
}

func TestReconcileDoesNotWriteStatusAfterPublishFailure(t *testing.T) {
	gateway := gatewayResource("gateway", 1, listener("http", 8080, ""))
	apiClient := newFakeClient(t, &gateway)
	wantErr := errors.New("publisher unavailable")
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return wantErr }))

	if _, err := r.Reconcile(t.Context(), sharedRequest()); !errors.Is(err, wantErr) {
		t.Fatalf("Reconcile() error = %v, want wrapped %v", err, wantErr)
	}

	var stored gatewayv1alpha1.Gateway
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: gateway.Name}, &stored); err != nil {
		t.Fatalf("Get(Gateway) error = %v", err)
	}
	if len(stored.Status.Conditions) != 0 {
		t.Errorf("Gateway conditions = %v, want none", stored.Status.Conditions)
	}
}

func TestReconcileReturnsStatusUpdateFailure(t *testing.T) {
	gateway := gatewayResource("gateway", 1, listener("http", 8080, ""))
	wantErr := errors.New("status unavailable")
	apiClient := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(&gateway).
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
				return wantErr
			},
		}).
		Build()
	var published bool
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
		published = true
		return nil
	}))

	if _, err := r.Reconcile(t.Context(), sharedRequest()); !errors.Is(err, wantErr) {
		t.Errorf("Reconcile() error = %v, want wrapped %v", err, wantErr)
	}
	if !published {
		t.Error("Reconcile() did not publish before updating Status")
	}
}

func TestHTTPRouteParentStatusesPreserveOwnershipAndTransitionTime(t *testing.T) {
	route := routeResource("route", 1, "edge", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	route.Generation = 7
	missing := gatewayv1alpha1.ParentReference{Name: "missing", SectionName: "http"}
	route.Spec.ParentRefs = append(route.Spec.ParentRefs, missing)
	transition := metav1.NewTime(time.Unix(100, 0))
	foreign := gatewayv1alpha1.RouteParentStatus{
		ParentRef:      route.Spec.ParentRefs[0],
		ControllerName: "other.example/controller",
		Conditions: []metav1.Condition{{
			Type:               "Accepted",
			Status:             metav1.ConditionFalse,
			Reason:             "Pending",
			LastTransitionTime: transition,
			ObservedGeneration: 6,
		}},
	}
	route.Status.Parents = []gatewayv1alpha1.RouteParentStatus{
		{
			ParentRef:      route.Spec.ParentRefs[0],
			ControllerName: controllerName,
			Conditions: []metav1.Condition{
				{
					Type:               "Accepted",
					Status:             metav1.ConditionTrue,
					Reason:             "Accepted",
					Message:            "attached",
					LastTransitionTime: transition,
					ObservedGeneration: 6,
				},
				{
					Type:               "ResolvedRefs",
					Status:             metav1.ConditionTrue,
					Reason:             "ResolvedRefs",
					Message:            "all Backend references resolve",
					LastTransitionTime: transition,
					ObservedGeneration: 6,
				},
			},
		},
		{ParentRef: gatewayv1alpha1.ParentReference{Name: "removed"}, ControllerName: controllerName},
		foreign,
	}
	apiClient := newFakeClient(t, &route)
	before := route.DeepCopy()
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))
	parents := []parentResolution{
		{routeName: route.Name, parentRef: route.Spec.ParentRefs[0], accepted: true, reason: "Accepted", message: "attached"},
		{routeName: route.Name, parentRef: missing, reason: "NoMatchingParent", message: "missing Gateway"},
	}
	if err := r.updateHTTPRouteStatuses(t.Context(), []gatewayv1alpha1.HTTPRoute{route}, parents); err != nil {
		t.Fatalf("updateHTTPRouteStatuses(route) error = %v, want nil", err)
	}
	var stored gatewayv1alpha1.HTTPRoute
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: route.Name}, &stored); err != nil {
		t.Fatalf("Get(HTTPRoute route) error = %v, want nil", err)
	}
	if len(stored.Status.Parents) != 3 {
		t.Fatalf("status parents = %+v, want two managed parents and one foreign parent", stored.Status.Parents)
	}
	for i, want := range []gatewayv1alpha1.ParentReference{route.Spec.ParentRefs[0], missing} {
		if stored.Status.Parents[i].ParentRef != want || stored.Status.Parents[i].ControllerName != controllerName {
			t.Errorf("managed parent = %+v, want %v owned by %s", stored.Status.Parents[i], want, controllerName)
		}
	}
	if len(stored.Status.Parents[0].Conditions) != 2 {
		t.Fatalf("accepted parent conditions = %+v, want Accepted and ResolvedRefs", stored.Status.Parents[0].Conditions)
	}
	assertCondition(t, stored.Status.Parents[0].Conditions, metav1.ConditionTrue, "Accepted", 7)
	resolved := meta.FindStatusCondition(stored.Status.Parents[0].Conditions, "ResolvedRefs")
	if resolved == nil || resolved.Status != metav1.ConditionTrue {
		t.Errorf("accepted parent ResolvedRefs = %+v, want True", resolved)
	}
	for _, condition := range stored.Status.Parents[0].Conditions {
		if condition.ObservedGeneration != 7 || !condition.LastTransitionTime.Equal(&transition) {
			t.Errorf("unchanged condition = %+v, want generation 7 and transition %v", condition, transition)
		}
	}
	assertCondition(t, stored.Status.Parents[1].Conditions, metav1.ConditionFalse, "NoMatchingParent", 7)
	if diff := cmp.Diff(foreign, stored.Status.Parents[2]); diff != "" {
		t.Errorf("foreign parent changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(before, &route); diff != "" {
		t.Errorf("input route changed (-want +got):\n%s", diff)
	}
	if err := r.updateHTTPRouteStatuses(t.Context(), []gatewayv1alpha1.HTTPRoute{stored}, parents); err != nil {
		t.Fatalf("repeat updateHTTPRouteStatuses(route) error = %v, want nil", err)
	}
	var repeated gatewayv1alpha1.HTTPRoute
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: route.Name}, &repeated); err != nil {
		t.Fatalf("Get(repeated HTTPRoute route) error = %v, want nil", err)
	}
	if repeated.ResourceVersion != stored.ResourceVersion {
		t.Errorf("idempotent status changed resourceVersion = %s, want %s", repeated.ResourceVersion, stored.ResourceVersion)
	}
}

func assertGatewayCondition(
	t *testing.T,
	apiClient client.Client,
	name string,
	status metav1.ConditionStatus,
	reason string,
	generation int64,
) {
	t.Helper()
	var gateway gatewayv1alpha1.Gateway
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: name}, &gateway); err != nil {
		t.Fatalf("Get(Gateway %q) error = %v", name, err)
	}
	assertCondition(t, gateway.Status.Conditions, status, reason, generation)
}

func assertHTTPRouteCondition(
	t *testing.T,
	apiClient client.Client,
	name string,
	status metav1.ConditionStatus,
	reason string,
	generation int64,
) {
	t.Helper()
	var route gatewayv1alpha1.HTTPRoute
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: name}, &route); err != nil {
		t.Fatalf("Get(HTTPRoute %q) error = %v", name, err)
	}
	if len(route.Status.Parents) != 1 {
		t.Fatalf("parents = %+v, want one", route.Status.Parents)
	}
	assertCondition(t, route.Status.Parents[0].Conditions, status, reason, generation)
}

func assertCondition(
	t *testing.T,
	conditions []metav1.Condition,
	status metav1.ConditionStatus,
	reason string,
	generation int64,
) {
	t.Helper()
	condition := meta.FindStatusCondition(conditions, gatewayv1alpha1.ConditionAccepted)
	if condition == nil {
		t.Fatal("Accepted condition not found")
	}
	if condition.Status != status || condition.Reason != reason || condition.ObservedGeneration != generation {
		t.Errorf("Accepted condition = %+v, want status %q, reason %q, generation %d", condition, status, reason, generation)
	}
}

func assertHTTPRouteResolvedRefs(t *testing.T, apiClient client.Client, name string, status metav1.ConditionStatus, reason string) {
	t.Helper()
	var route gatewayv1alpha1.HTTPRoute
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: name}, &route); err != nil {
		t.Fatal(err)
	}
	if len(route.Status.Parents) != 1 {
		t.Fatalf("parents = %+v, want one", route.Status.Parents)
	}
	condition := meta.FindStatusCondition(route.Status.Parents[0].Conditions, "ResolvedRefs")
	if condition == nil || condition.Status != status || condition.Reason != reason {
		t.Errorf("ResolvedRefs = %+v, want %s/%s", condition, status, reason)
	}
}
