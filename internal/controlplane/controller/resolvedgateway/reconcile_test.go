package resolvedgateway

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

func TestReconcileCreatesOrUpdatesResolvedGateway(t *testing.T) {
	gateway := gatewayFixture("public-edge")
	route := routeFixture("catalog-route", "public-edge", "catalog-backend")
	backend := backendFixture("catalog-backend")
	certificate := certificateFixture("cert-a")
	authPolicy := authPolicyFixture("route-auth", "Route", "catalog-route")
	trafficPolicy := trafficPolicyFixture("backend-traffic", "Backend", "catalog-backend")

	bundle := &ResourceBundle{
		Gateway:                gateway,
		Routes:                 []*gatewayv1alpha1.Route{route},
		Backends:               []*gatewayv1alpha1.Backend{backend},
		Certificates:           []*gatewayv1alpha1.Certificate{certificate},
		RouteAuthPolicies:      map[string][]*policyv1alpha1.AuthPolicy{"catalog-route": {authPolicy}},
		BackendTrafficPolicies: map[string][]*policyv1alpha1.TrafficPolicy{"catalog-backend": {trafficPolicy}},
	}

	persister := &recordingPersister{}
	status := &recordingStatusWriter{}
	controller := &Controller{loader: staticLoader{bundle: bundle}, persister: persister, status: status}

	if err := controller.Reconcile(context.Background(), shared.NewObjectKey("", "public-edge")); err != nil {
		t.Fatalf("reconcile error: %v", err)
	}

	if persister.lastResolvedGateway == nil {
		t.Fatal("expected resolvedgateway to be persisted")
	}
	if got := persister.lastResolvedGateway.Spec.GatewayRef.Name; got != "public-edge" {
		t.Fatalf("unexpected gateway ref: %q", got)
	}
	assertConditionTrue(t, persister.lastResolvedGateway.Status.Conditions, "Accepted")
	assertConditionTrue(t, persister.lastResolvedGateway.Status.Conditions, "Resolved")
	assertConditionTrue(t, gateway.Status.Conditions, "Accepted")
	assertConditionTrue(t, gateway.Status.Conditions, "Resolved")
	assertConditionTrue(t, route.Status.Conditions, "Accepted")
	assertConditionTrue(t, route.Status.Conditions, "Resolved")
	if status.failureErr != nil {
		t.Fatalf("unexpected failure status call: %v", status.failureErr)
	}
}

type staticLoader struct{ bundle *ResourceBundle }

func (l staticLoader) Load(shared.ObjectKey) (*ResourceBundle, error) { return l.bundle, nil }

type recordingPersister struct {
	lastResolvedGateway *gatewayv1alpha1.ResolvedGateway
}

func (p *recordingPersister) Upsert(_ context.Context, rg *gatewayv1alpha1.ResolvedGateway) (*gatewayv1alpha1.ResolvedGateway, error) {
	p.lastResolvedGateway = rg.DeepCopy()
	return p.lastResolvedGateway, nil
}

type recordingStatusWriter struct {
	failureErr error
}

func (w *recordingStatusWriter) MarkSuccess(
	_ context.Context,
	gateway *gatewayv1alpha1.Gateway,
	routes []*gatewayv1alpha1.Route,
	_ []*gatewayv1alpha1.Backend,
	_ []*gatewayv1alpha1.Certificate,
	_ []*policyv1alpha1.AuthPolicy,
	_ []*policyv1alpha1.TrafficPolicy,
	rg *gatewayv1alpha1.ResolvedGateway,
) error {
	gateway.Status.ObservedGeneration = gateway.Generation
	gateway.Status.Conditions = []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}, {Type: "Resolved", Status: metav1.ConditionTrue}}
	for _, route := range routes {
		route.Status.ObservedGeneration = route.Generation
		route.Status.Conditions = []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}, {Type: "Resolved", Status: metav1.ConditionTrue}}
	}
	rg.Status.ObservedGeneration = gateway.Generation
	rg.Status.Conditions = []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}, {Type: "Resolved", Status: metav1.ConditionTrue}}
	return nil
}

func (w *recordingStatusWriter) MarkFailure(_ context.Context, _ shared.ObjectKey, err error) error {
	w.failureErr = err
	return nil
}

func gatewayFixture(name string) *gatewayv1alpha1.Gateway {
	obj := gateway(name)
	obj.Generation = 1
	return obj
}

func routeFixture(name, parent, backendName string) *gatewayv1alpha1.Route {
	obj := route(name, parent, backendName)
	obj.Generation = 1
	return obj
}

func backendFixture(name string) *gatewayv1alpha1.Backend {
	obj := backend(name)
	obj.Generation = 1
	return obj
}

func certificateFixture(name string) *gatewayv1alpha1.Certificate {
	obj := certificate(name)
	obj.Generation = 1
	return obj
}

func authPolicyFixture(name, kind, target string) *policyv1alpha1.AuthPolicy {
	obj := authPolicy(name, kind, target)
	obj.Generation = 1
	return obj
}

func trafficPolicyFixture(name, kind, target string) *policyv1alpha1.TrafficPolicy {
	obj := trafficPolicy(name, kind, target)
	obj.Generation = 1
	return obj
}

func assertConditionTrue(t *testing.T, conditions []metav1.Condition, conditionType string) {
	t.Helper()
	for _, condition := range conditions {
		if condition.Type == conditionType {
			if condition.Status != metav1.ConditionTrue {
				t.Fatalf("condition %s status = %s, want True", conditionType, condition.Status)
			}
			return
		}
	}
	t.Fatalf("condition %s not found", conditionType)
}
