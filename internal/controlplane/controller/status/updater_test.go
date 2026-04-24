package status

import (
	"context"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	clientsetfake "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestMarkFailureMarksRelatedResourcesFalse(t *testing.T) {
	gateway := gatewayFixture("public-edge")
	route := routeFixture("catalog-route", "public-edge", "catalog-backend")
	backend := backendFixture("catalog-backend")
	certificate := certificateFixture("cert-a")
	authPolicy := authPolicyFixture("route-auth", "Route", "catalog-route")
	trafficPolicy := trafficPolicyFixture("backend-traffic", "Backend", "catalog-backend")

	client := newStatusTestClient(gateway, route, backend, certificate, authPolicy, trafficPolicy)
	updater := NewUpdater(client)

	if err := updater.MarkFailure(context.Background(), shared.NewObjectKey("", gateway.Name), fmt.Errorf("resolve failed")); err != nil {
		t.Fatalf("MarkFailure() error = %v", err)
	}

	assertUpdatedResource(t, client.Actions(), "routes")
	assertUpdatedResource(t, client.Actions(), "backends")
	assertUpdatedResource(t, client.Actions(), "certificates")
	assertUpdatedResource(t, client.Actions(), "authpolicies")
	assertUpdatedResource(t, client.Actions(), "trafficpolicies")
}

func TestMarkFailureContinuesUpdatingRelatedResourcesAfterOneWriteFails(t *testing.T) {
	gateway := gatewayFixture("public-edge")
	route := routeFixture("catalog-route", "public-edge", "catalog-backend")
	backend := backendFixture("catalog-backend")
	certificate := certificateFixture("cert-a")
	authPolicy := authPolicyFixture("route-auth", "Route", "catalog-route")
	trafficPolicy := trafficPolicyFixture("backend-traffic", "Backend", "catalog-backend")

	client := newStatusTestClient(gateway, route, backend, certificate, authPolicy, trafficPolicy)
	client.PrependReactor("update", "routes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("route status write failed")
	})
	updater := NewUpdater(client)

	err := updater.MarkFailure(context.Background(), shared.NewObjectKey("", gateway.Name), fmt.Errorf("resolve failed"))
	if err == nil {
		t.Fatal("expected MarkFailure() to return aggregated error")
	}

	assertUpdatedResource(t, client.Actions(), "backends")
	assertUpdatedResource(t, client.Actions(), "certificates")
	assertUpdatedResource(t, client.Actions(), "authpolicies")
	assertUpdatedResource(t, client.Actions(), "trafficpolicies")
}

func gatewayFixture(name string) *gatewayv1alpha1.Gateway {
	return &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: gatewayv1alpha1.GatewaySpec{Listeners: []gatewayv1alpha1.GatewayListener{{Name: "https", Protocol: "HTTPS", Port: 443, TLS: &gatewayv1alpha1.GatewayTLSConfig{CertificateRef: &gatewayv1alpha1.LocalObjectReference{Name: "cert-a"}}}}}}
}

func routeFixture(name, parent, backendName string) *gatewayv1alpha1.Route {
	return &gatewayv1alpha1.Route{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: gatewayv1alpha1.RouteSpec{ParentRefs: []gatewayv1alpha1.ParentReference{{Name: parent}}, Rules: []gatewayv1alpha1.RouteRule{{BackendRefs: []gatewayv1alpha1.BackendRef{{Name: backendName, Port: 8080}}}}}}
}

func backendFixture(name string) *gatewayv1alpha1.Backend {
	return &gatewayv1alpha1.Backend{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: gatewayv1alpha1.BackendSpec{Protocol: gatewayv1alpha1.BackendProtocolHTTP, DefaultPort: 8080, Static: &gatewayv1alpha1.StaticBackendSpec{Endpoints: []gatewayv1alpha1.BackendEndpoint{{Address: "10.0.0.1", Port: 8080, Healthy: true}}}}}
}

func certificateFixture(name string) *gatewayv1alpha1.Certificate {
	return &gatewayv1alpha1.Certificate{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: gatewayv1alpha1.CertificateSpec{SecretRef: gatewayv1alpha1.LocalObjectReference{Name: "secret-a"}, Domains: []string{"example.com"}}}
}

func authPolicyFixture(name, kind, target string) *policyv1alpha1.AuthPolicy {
	return &policyv1alpha1.AuthPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: policyv1alpha1.AuthPolicySpec{TargetRefs: []policyv1alpha1.TargetReference{{Kind: kind, Name: target}}, Type: "JWT", JWT: &policyv1alpha1.JWTAuthSpec{Issuer: "issuer-a"}}}
}

func trafficPolicyFixture(name, kind, target string) *policyv1alpha1.TrafficPolicy {
	return &policyv1alpha1.TrafficPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: policyv1alpha1.TrafficPolicySpec{TargetRefs: []policyv1alpha1.TargetReference{{Kind: kind, Name: target}}, Timeout: &policyv1alpha1.TimeoutSpec{Duration: "5s"}}}
}

func assertUpdatedResource(t *testing.T, actions []k8stesting.Action, resource string) {
	t.Helper()
	for _, action := range actions {
		verb := action.GetVerb()
		if (verb == "update" || verb == "patch") && action.GetResource().Resource == resource {
			return
		}
	}
	t.Fatalf("expected update action for %s, got %#v", resource, actions)
}

func newStatusTestClient(
	gateway *gatewayv1alpha1.Gateway,
	route *gatewayv1alpha1.Route,
	backend *gatewayv1alpha1.Backend,
	certificate *gatewayv1alpha1.Certificate,
	authPolicy *policyv1alpha1.AuthPolicy,
	trafficPolicy *policyv1alpha1.TrafficPolicy,
) *clientsetfake.Clientset {
	client := clientsetfake.NewSimpleClientset()

	getObjects := map[string]map[string]runtime.Object{
		"gateways": {
			gateway.Name: gateway.DeepCopy(),
		},
	}
	listObjects := map[string]runtime.Object{
		"gateways":        &gatewayv1alpha1.GatewayList{Items: []gatewayv1alpha1.Gateway{*gateway.DeepCopy()}},
		"routes":          &gatewayv1alpha1.RouteList{Items: []gatewayv1alpha1.Route{*route.DeepCopy()}},
		"backends":        &gatewayv1alpha1.BackendList{Items: []gatewayv1alpha1.Backend{*backend.DeepCopy()}},
		"certificates":    &gatewayv1alpha1.CertificateList{Items: []gatewayv1alpha1.Certificate{*certificate.DeepCopy()}},
		"authpolicies":    &policyv1alpha1.AuthPolicyList{Items: []policyv1alpha1.AuthPolicy{*authPolicy.DeepCopy()}},
		"trafficpolicies": &policyv1alpha1.TrafficPolicyList{Items: []policyv1alpha1.TrafficPolicy{*trafficPolicy.DeepCopy()}},
	}

	client.PrependReactor("get", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		getAction, ok := action.(k8stesting.GetAction)
		if !ok {
			return false, nil, nil
		}
		byName, ok := getObjects[action.GetResource().Resource]
		if !ok {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: action.GetResource().Group, Resource: action.GetResource().Resource}, getAction.GetName())
		}
		obj, ok := byName[getAction.GetName()]
		if !ok {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: action.GetResource().Group, Resource: action.GetResource().Resource}, getAction.GetName())
		}
		return true, obj.DeepCopyObject(), nil
	})

	client.PrependReactor("list", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		obj, ok := listObjects[action.GetResource().Resource]
		if !ok {
			return false, nil, nil
		}
		return true, obj.DeepCopyObject(), nil
	})

	client.PrependReactor("create", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		return true, createAction.GetObject(), nil
	})

	client.PrependReactor("update", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateAction, ok := action.(k8stesting.UpdateAction)
		if !ok {
			return false, nil, nil
		}
		return true, updateAction.GetObject(), nil
	})

	return client
}
