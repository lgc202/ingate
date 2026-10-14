package gateway

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestResolveParentsAndMissingBackend(t *testing.T) {
	gateways := []gatewayv1alpha1.Gateway{gatewayResource("edge", 1, listener("a", 8080, ""), listener("b", 9090, ""))}
	route := routeResource("public", 1, "edge", "", "missing", gatewayv1alpha1.PathMatchPrefix)
	route.Spec.ParentRefs = append(route.Spec.ParentRefs,
		gatewayv1alpha1.ParentReference{Name: "edge", SectionName: "absent"},
		gatewayv1alpha1.ParentReference{Name: "unknown"},
	)
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
	result := resolve(
		[]gatewayv1alpha1.Gateway{gateway},
		routes,
		[]gatewayv1alpha1.Backend{backendResource("backend"), backendResource("unused")},
	)
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
	route.Spec.Rules[0].Matches = []gatewayv1alpha1.HTTPRouteMatch{
		{},
		{Path: &gatewayv1alpha1.PathMatch{Type: gatewayv1alpha1.PathMatchExact, Value: "/api/"}},
	}
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
		if got := defaultHTTPRoutePath(tt.path); got != tt.want {
			t.Errorf("defaultHTTPRoutePath(%+v) = %+v, want %+v", tt.path, got, tt.want)
		}
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
		WithObjects(&gateway, &route, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}}).
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
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))

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
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))
	parents := []httpRouteParentResolution{
		{
			routeKey:  client.ObjectKeyFromObject(&route),
			parentRef: route.Spec.ParentRefs[0],
			accepted:  true,
			reason:    "Accepted",
			message:   "attached",
		},
		{
			routeKey:  client.ObjectKeyFromObject(&route),
			parentRef: missing,
			reason:    "NoMatchingParent",
			message:   "missing Gateway",
		},
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
	assertAcceptedCondition(t, stored.Status.Parents[0].Conditions, metav1.ConditionTrue, "Accepted", 7)
	resolved := meta.FindStatusCondition(stored.Status.Parents[0].Conditions, "ResolvedRefs")
	if resolved == nil || resolved.Status != metav1.ConditionTrue {
		t.Errorf("accepted parent ResolvedRefs = %+v, want True", resolved)
	}
	for _, condition := range stored.Status.Parents[0].Conditions {
		if condition.ObservedGeneration != 7 || !condition.LastTransitionTime.Equal(&transition) {
			t.Errorf("unchanged condition = %+v, want generation 7 and transition %v", condition, transition)
		}
	}
	assertAcceptedCondition(t, stored.Status.Parents[1].Conditions, metav1.ConditionFalse, "NoMatchingParent", 7)
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
	assertAcceptedCondition(t, route.Status.Parents[0].Conditions, status, reason, generation)
}

func assertHTTPRouteResolvedRefs(
	t *testing.T,
	apiClient client.Client,
	name string,
	status metav1.ConditionStatus,
	reason string,
) {
	t.Helper()
	var route gatewayv1alpha1.HTTPRoute
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: name}, &route); err != nil {
		t.Fatalf("Get(HTTPRoute %q) error = %v", name, err)
	}
	if len(route.Status.Parents) != 1 {
		t.Fatalf("parents = %+v, want one", route.Status.Parents)
	}
	condition := meta.FindStatusCondition(route.Status.Parents[0].Conditions, "ResolvedRefs")
	if condition == nil || condition.Status != status || condition.Reason != reason {
		t.Errorf("ResolvedRefs = %+v, want %s/%s", condition, status, reason)
	}
}
