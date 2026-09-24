package gateway

import (
	"context"
	"errors"
	"testing"

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
	upstream := upstreamResource("backend")
	apiClient := newFakeClient(t, &first, &conflicting, &valid, &rejected, &upstream)
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	assertGatewayCondition(t, apiClient, "first", metav1.ConditionTrue, "Accepted", 3)
	assertGatewayCondition(t, apiClient, "conflicting", metav1.ConditionFalse, "ListenerConflict", 4)
	assertRouteCondition(t, apiClient, "valid", metav1.ConditionTrue, "Accepted", 5)
	assertRouteCondition(t, apiClient, "rejected", metav1.ConditionFalse, "GatewayRejected", 6)

	var stored gatewayv1alpha1.Upstream
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: "backend"}, &stored); err != nil {
		t.Fatalf("Get(Upstream) error = %v", err)
	}
	if len(stored.Status.Conditions) != 0 {
		t.Errorf("Upstream conditions = %v, want none", stored.Status.Conditions)
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
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.Route{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, writer client.Client, name string, object client.Object, options ...client.SubResourceUpdateOption) error {
				updates++
				return writer.SubResource(name).Update(ctx, object, options...)
			},
		}).
		Build()
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))

	for range 2 {
		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(missing Upstream) error = %v", err)
		}
	}
	if updates != 2 {
		t.Errorf("status updates before Upstream exists = %d, want 2", updates)
	}
	assertRouteCondition(t, apiClient, "route", metav1.ConditionFalse, "UpstreamMissing", 7)

	upstream := upstreamResource("backend")
	if err := apiClient.Create(t.Context(), &upstream); err != nil {
		t.Fatalf("Create(Upstream) error = %v", err)
	}
	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(available Upstream) error = %v", err)
	}
	if updates != 3 {
		t.Errorf("status updates after Upstream appears = %d, want 3", updates)
	}
	assertRouteCondition(t, apiClient, "route", metav1.ConditionTrue, "Accepted", 7)
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

func TestRejectionMessage(t *testing.T) {
	tests := []struct {
		name   string
		reason routeRejectionReason
		want   string
	}{
		{name: "missing Gateway", reason: gatewayMissing, want: `Gateway "example" does not exist`},
		{name: "rejected Gateway", reason: gatewayRejected, want: `Gateway "example" is not accepted`},
		{name: "missing listener", reason: listenerMissing, want: `listener "example" does not exist on the Gateway`},
		{name: "missing Upstream", reason: upstreamMissing, want: `Upstream "example" does not exist`},
		{name: "conflicting match", reason: matchConflict, want: `path match conflicts with Route "example"`},
		{name: "unrecognized reason", reason: "Other", want: "Route was not accepted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rejection := routeRejection{reason: tt.reason, relatedName: "example"}
			if got := rejectionMessage(rejection); got != tt.want {
				t.Errorf("rejectionMessage(%q) = %q, want %q", tt.reason, got, tt.want)
			}
		})
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

func assertRouteCondition(
	t *testing.T,
	apiClient client.Client,
	name string,
	status metav1.ConditionStatus,
	reason string,
	generation int64,
) {
	t.Helper()
	var route gatewayv1alpha1.Route
	if err := apiClient.Get(t.Context(), types.NamespacedName{Name: name}, &route); err != nil {
		t.Fatalf("Get(Route %q) error = %v", name, err)
	}
	assertCondition(t, route.Status.Conditions, status, reason, generation)
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
