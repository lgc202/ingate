package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type publisherFunc func(context.Context, dataplane.Config) error

func TestReconcilePublishesAcceptedResources(t *testing.T) {
	first := gatewayResource("first", 1, listener("http", 8080, ""))
	conflicting := gatewayResource("conflicting", 2, listener("http", 8080, ""))
	valid := routeResource("valid", 1, "first", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	rejected := routeResource("rejected", 2, "conflicting", "http", "backend", gatewayv1alpha1.PathMatchPrefix)
	backend := backendResource("backend")
	apiClient := newFakeClient(t, &conflicting, &first, &rejected, &valid, &backend)

	var published dataplane.Config
	var calls int
	r := newTestReconciler(t, apiClient, publisherFunc(func(_ context.Context, config dataplane.Config) error {
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
					Name: "valid/rule/0/match/0", PathType: dataplane.PathMatchPrefix,
					Path: "/api", BackendName: "backend",
				}},
			}},
		}},
		Backends: []dataplane.Backend{{
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
	r := newTestReconciler(t, apiClient, publisherFunc(func(_ context.Context, config dataplane.Config) error {
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

func TestReconcileDoesNotListRawResources(t *testing.T) {
	snapshot := aiResourceSnapshot()
	apiClient := aiResourceClient(t, snapshot)
	reads := interceptor.NewClient(apiClient, interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			t.Error("Reconcile listed raw resources instead of reading compiled collections")

			return errors.New("raw List is forbidden")
		},
	})
	var publishes int
	r := New(reads, apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
		publishes++

		return nil
	}), "control-plane")
	setTestResources(t, r, snapshot)

	if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
		t.Fatalf("Reconcile(compiled collections) error = %v, want nil", err)
	}

	if publishes != 1 {
		t.Errorf("Reconcile(compiled collections) HTTP publishes = %d, want 1", publishes)
	}

	if got := storedAIConfigMap(t, apiClient); got.Data[aidataplane.ConfigMapDataKey] != string(aiConfigPayload(t, snapshot)) {
		t.Errorf("Reconcile(compiled collections) AI payload = %s, want complete compiled payload", got.Data[aidataplane.ConfigMapDataKey])
	}
}

func TestReconcileReturnsPublishFailure(t *testing.T) {
	wantErr := errors.New("publisher unavailable")
	r := newTestReconciler(t, newFakeClient(t), publisherFunc(func(context.Context, dataplane.Config) error {
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
		&gatewayv1alpha1.HTTPRoute{},
		&gatewayv1alpha1.Backend{},
		&gatewayv1alpha1.AIBackend{},
		&gatewayv1alpha1.ModelRoute{},
		&gatewayv1alpha1.MCPRoute{},
		&corev1.ConfigMap{},
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
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return nil }))

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

func TestReconcileDoesNotWriteStatusAfterPublishFailure(t *testing.T) {
	gateway := gatewayResource("gateway", 1, listener("http", 8080, ""))
	apiClient := newFakeClient(t, &gateway)
	wantErr := errors.New("publisher unavailable")
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error { return wantErr }))

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
		WithObjects(&gateway, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}}).
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
				return wantErr
			},
		}).
		Build()
	var published bool
	r := newTestReconciler(t, apiClient, publisherFunc(func(context.Context, dataplane.Config) error {
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

func (f publisherFunc) Publish(ctx context.Context, config dataplane.Config) error {
	return f(ctx, config)
}

func (f publisherFunc) PublishAI(context.Context, aidataplane.Config, []*aidataplanev1.Resource) error {
	return nil
}

func newTestReconciler(
	t *testing.T,
	apiClient client.Client,
	publisher Publisher,
) *Reconciler {
	t.Helper()

	r := New(apiClient, apiClient, publisher, "control-plane")
	loadTestCollections(t, r)

	return r
}

// loadTestCollections 将当前 API 场景装入只读集合，供不启动 Informer 的协调测试使用。
func loadTestCollections(t *testing.T, reconciler *Reconciler) {
	t.Helper()
	fixtures := readTestResources(t, reconciler.client)
	setTestResources(t, reconciler, fixtures)
}

func readTestResources(t *testing.T, apiClient client.Reader) resourceFixtures {
	t.Helper()
	var fixtures resourceFixtures
	for _, list := range []client.ObjectList{
		&fixtures.gateways, &fixtures.httpRoutes, &fixtures.backends,
		&fixtures.aiBackends, &fixtures.modelRoutes, &fixtures.mcpRoutes,
	} {
		if err := apiClient.List(t.Context(), list); err != nil {
			t.Fatalf("List(%T) for test collections error = %v, want nil", list, err)
		}
	}

	return fixtures
}

func newFakeClient(t *testing.T, objects ...client.Object) client.WithWatch {
	t.Helper()

	objects = append(objects, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "control-plane"}})

	return fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&gatewayv1alpha1.Gateway{}, &gatewayv1alpha1.HTTPRoute{}).
		Build()
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("core AddToScheme() error = %v", err)
	}

	if err := gatewayv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	return scheme
}

func assertAcceptedCondition(
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
