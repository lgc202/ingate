package reconcile

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	controllerstatus "github.com/lgc202/ingate/internal/controller/status"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	fakeclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/fake"
)

func TestResourceListersBuildImmutableSet(t *testing.T) {
	client := fakeclient.NewSimpleClientset()
	r, err := New(client, 0, nil, controllerstatus.NewRuntime(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(r.queue.ShutDown)

	informers := r.factory.Gateway().V1()
	gatewayB := &gatewayv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: "gateway-b"},
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Name: "http", Protocol: gatewayv1.ProtocolHTTP, Port: 8080}},
		},
	}
	gatewayA := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-a"}}
	route := &gatewayv1.Route{ObjectMeta: metav1.ObjectMeta{Name: "route-a"}}
	upstream := &gatewayv1.Upstream{ObjectMeta: metav1.ObjectMeta{Name: "upstream-a"}}
	rateLimitPolicy := &gatewayv1.RateLimitPolicy{ObjectMeta: metav1.ObjectMeta{Name: "rate-a"}}
	accessControlPolicy := &gatewayv1.AccessControlPolicy{ObjectMeta: metav1.ObjectMeta{Name: "access-a"}}
	policyBinding := &gatewayv1.PolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "binding-a"}}

	addToStore(t, informers.Gateways().Informer().GetStore(), gatewayB)
	addToStore(t, informers.Gateways().Informer().GetStore(), gatewayA)
	addToStore(t, informers.Routes().Informer().GetStore(), route)
	addToStore(t, informers.Upstreams().Informer().GetStore(), upstream)
	addToStore(t, informers.RateLimitPolicies().Informer().GetStore(), rateLimitPolicy)
	addToStore(t, informers.AccessControlPolicies().Informer().GetStore(), accessControlPolicy)
	addToStore(t, informers.PolicyBindings().Informer().GetStore(), policyBinding)

	resources, err := r.resources.build()
	if err != nil {
		t.Fatalf("build() error = %v", err)
	}
	if got, want := len(resources.Gateways), 2; got != want {
		t.Fatalf("len(ResourceSet.Gateways) = %d, want %d", got, want)
	}
	if got, want := resources.Gateways[0].Name, gatewayA.Name; got != want {
		t.Errorf("ResourceSet.Gateways[0].Name = %q, want %q", got, want)
	}
	if got, want := len(resources.Routes), 1; got != want {
		t.Errorf("len(ResourceSet.Routes) = %d, want %d", got, want)
	}
	if got, want := len(resources.Upstreams), 1; got != want {
		t.Errorf("len(ResourceSet.Upstreams) = %d, want %d", got, want)
	}
	if got, want := len(resources.RateLimitPolicies), 1; got != want {
		t.Errorf("len(ResourceSet.RateLimitPolicies) = %d, want %d", got, want)
	}
	if got, want := len(resources.AccessControlPolicies), 1; got != want {
		t.Errorf("len(ResourceSet.AccessControlPolicies) = %d, want %d", got, want)
	}
	if got, want := len(resources.PolicyBindings), 1; got != want {
		t.Errorf("len(ResourceSet.PolicyBindings) = %d, want %d", got, want)
	}

	gatewayB.Spec.Listeners[0].Name = "changed-in-cache"
	if got, want := resources.Gateways[1].Spec.Listeners[0].Name, "http"; got != want {
		t.Errorf("ResourceSet Gateway listener name = %q after cache mutation, want %q", got, want)
	}
}

func TestResourceListersBuildReturnsListerError(t *testing.T) {
	listers := resourceListers{gateways: failingGatewayLister{}}
	if _, err := listers.build(); err == nil {
		t.Fatal("build() error = nil, want lister error")
	}
}

type failingGatewayLister struct{}

func (failingGatewayLister) List(labels.Selector) ([]*gatewayv1.Gateway, error) {
	return nil, errors.New("list failed")
}

func (failingGatewayLister) Get(string) (*gatewayv1.Gateway, error) {
	return nil, errors.New("get failed")
}

type store interface {
	Add(any) error
}

func addToStore(t *testing.T, target store, object any) {
	t.Helper()
	if err := target.Add(object); err != nil {
		t.Fatalf("store.Add(%T) error = %v", object, err)
	}
}
