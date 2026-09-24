package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestReconcilePublishesAcceptedResources(t *testing.T) {
	first := gatewayResource("first", 1, listener("http", 8080, ""))
	conflicting := gatewayResource("conflicting", 2, listener("http", 8080, ""))
	valid := routeResource("valid", 1, "first", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	rejected := routeResource("rejected", 2, "conflicting", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	upstream := upstreamResource("backend")
	apiClient := newFakeClient(t, &conflicting, &first, &rejected, &valid, &upstream)

	var published dataplane.Config
	var calls int
	r := New(apiClient, publisherFunc(func(_ context.Context, config dataplane.Config) error {
		published = config
		calls++
		return nil
	}))
	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(shared request) error = %v", err)
	}

	want := dataplane.Config{
		Listeners: []dataplane.Listener{{
			Port: 8080,
			Hosts: []dataplane.Host{{
				Name: "gateway/first/listener/http",
				Routes: []dataplane.Route{{
					Name: "valid", PathType: dataplane.PathMatchPrefix,
					Path: "/api", UpstreamName: "backend",
				}},
			}},
		}},
		Upstreams: []dataplane.Upstream{{
			Name: "backend",
			Endpoints: []dataplane.Endpoint{{
				Address: "127.0.0.1", Port: 8081, Weight: 1,
			}},
		}},
	}

	if diff := cmp.Diff(want, published); diff != "" {
		t.Errorf("Reconcile(shared request) published configuration mismatch (-want +got):\n%s", diff)
	}
	if calls != 1 {
		t.Errorf("Reconcile(shared request) publish calls = %d, want 1", calls)
	}
}

func TestReconcilePublishesEmptyConfiguration(t *testing.T) {
	apiClient := newFakeClient(t)
	var published dataplane.Config
	var calls int
	r := New(apiClient, publisherFunc(func(_ context.Context, config dataplane.Config) error {
		published = config
		calls++
		return nil
	}))
	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(empty resources) error = %v", err)
	}
	if calls != 1 {
		t.Errorf("Reconcile(empty resources) publish calls = %d, want 1", calls)
	}
	if diff := cmp.Diff(dataplane.Config{}, published); diff != "" {
		t.Errorf("Reconcile(empty resources) published configuration mismatch (-want +got):\n%s", diff)
	}
}

func TestReconcileDoesNotPublishAfterListFailure(t *testing.T) {
	wantErr := errors.New("list unavailable")
	scheme := newTestScheme(t)
	apiClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
				return wantErr
			},
		}).
		Build()

	var calls int
	r := New(apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
		calls++
		return nil
	}))

	_, err := r.Reconcile(t.Context(), sharedRequest())
	if !errors.Is(err, wantErr) {
		t.Errorf("Reconcile(list failure) error = %v, want wrapped %v", err, wantErr)
	}
	if calls != 0 {
		t.Errorf("Reconcile(list failure) publish calls = %d, want 0", calls)
	}
}

func TestReconcileReturnsPublishFailure(t *testing.T) {
	wantErr := errors.New("publisher unavailable")
	r := New(newFakeClient(t), publisherFunc(func(context.Context, dataplane.Config) error {
		return wantErr
	}))

	_, err := r.Reconcile(t.Context(), sharedRequest())
	if !errors.Is(err, wantErr) {
		t.Errorf("Reconcile(publish failure) error = %v, want wrapped %v", err, wantErr)
	}
}

func TestSharedRequestSources(t *testing.T) {
	want := []reconcile.Request{sharedRequest()}
	for _, object := range []client.Object{
		&gatewayv1alpha1.Gateway{},
		&gatewayv1alpha1.Route{},
		&gatewayv1alpha1.Upstream{},
	} {
		if diff := cmp.Diff(want, sharedRequests(t.Context(), object)); diff != "" {
			t.Errorf("sharedRequests(%T) mismatch (-want +got):\n%s", object, diff)
		}
	}

	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	t.Cleanup(queue.ShutDown)
	if err := enqueueInitialReconcile(t.Context(), queue); err != nil {
		t.Fatalf("enqueueInitialReconcile() error = %v", err)
	}
	if queue.Len() != 1 {
		t.Fatalf("enqueueInitialReconcile() queue length = %d, want 1", queue.Len())
	}
	request, shutdown := queue.Get()
	if shutdown {
		t.Fatal("enqueueInitialReconcile() shut down the request queue")
	}
	queue.Done(request)
	if request != sharedRequest() {
		t.Errorf("enqueueInitialReconcile() request = %v, want %v", request, sharedRequest())
	}
}

type publisherFunc func(context.Context, dataplane.Config) error

func (f publisherFunc) Publish(ctx context.Context, config dataplane.Config) error {
	return f(ctx, config)
}

func newFakeClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.Route{}).
		Build()
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := gatewayv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	return scheme
}
