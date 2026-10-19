package gateway

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"istio.io/istio/pkg/kube/krt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	informertesting "k8s.io/client-go/tools/cache/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

func TestCompileNetworkBackend(t *testing.T) {
	backend := &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "orders", Generation: 3, DeletionTimestamp: new(metav1.Unix(2, 0))},
		Spec: gatewayv1alpha1.BackendSpec{
			DisplayName: "订单后端",
			Endpoints: []gatewayv1alpha1.Endpoint{
				{Address: "z.example.com", Port: 443, Weight: 2},
				{Address: "a.example.com", Port: 8443, Weight: 1},
			},
			TLS: &gatewayv1alpha1.BackendTLSSettings{SNI: "orders.example.com", WellKnownCACertificates: "System"},
		},
		Status: gatewayv1alpha1.BackendStatus{Conditions: []metav1.Condition{{Type: "Accepted"}}},
	}
	before := backend.DeepCopy()
	want := &networkBackend{
		key: client.ObjectKey{Namespace: "team-a", Name: "orders"},
		endpoints: []dataplane.Endpoint{
			{Address: "a.example.com", Port: 8443, Weight: 1},
			{Address: "z.example.com", Port: 443, Weight: 2},
		},
		tls:      &gatewayv1alpha1.BackendTLSSettings{SNI: "orders.example.com", WellKnownCACertificates: "System"},
		deleting: true,
	}

	got := compileNetworkBackend(backend)

	if diff := cmp.Diff(want, got, cmp.AllowUnexported(networkBackend{})); diff != "" {
		t.Errorf("compileNetworkBackend() mismatch (-want +got):\n%s", diff)
	}

	wantHTTP := dataplane.Backend{
		Name: "team-a/orders", Endpoints: want.endpoints,
		TLS: &dataplane.BackendTLSSettings{SNI: "orders.example.com"},
	}
	if diff := cmp.Diff(wantHTTP, got.httpBackend()); diff != "" {
		t.Errorf("httpBackend() mismatch (-want +got):\n%s", diff)
	}

	wantAI := aidataplane.NetworkBackend{
		Name: "team-a/orders",
		Endpoints: []aidataplane.Endpoint{
			{Address: "a.example.com", Port: 8443, Weight: 1},
			{Address: "z.example.com", Port: 443, Weight: 2},
		},
		TLS: &aidataplane.BackendTLSSettings{SNI: "orders.example.com", WellKnownCACertificates: "System"},
	}
	if diff := cmp.Diff(wantAI, got.aiBackend()); diff != "" {
		t.Errorf("aiBackend() mismatch (-want +got):\n%s", diff)
	}

	got.endpoints[0].Address = "changed.example.com"
	got.tls.SNI = "changed.example.com"
	if diff := cmp.Diff(before, backend); diff != "" {
		t.Errorf("network result shares source fields (-want +got):\n%s", diff)
	}
}

func TestNetworkBackendIdentityIncludesNamespace(t *testing.T) {
	var backends []*networkBackend
	for _, namespace := range []string{"team-a", "team-b"} {
		backend := backendResource("orders")
		backend.Namespace = namespace
		backends = append(backends, compileNetworkBackend(&backend))
	}

	collection := krt.NewStaticCollection(nil, backends, krt.WithStop(t.Context().Done()))

	if got := len(collection.List()); got != 2 {
		t.Fatalf("network collection size = %d, want 2 namespaced Backends", got)
	}

	for _, backend := range backends {
		if got := collection.GetKey(backend.key.Namespace + "/orders"); got == nil || *got != backend {
			t.Errorf("GetKey(%s/orders) = %v, want matching Backend", backend.key.Namespace, got)
		}
	}

	if got := collection.GetKey("orders"); got != nil {
		t.Errorf("GetKey(orders) = %v, want no unscoped Backend", got)
	}
}

func TestNetworkBackendPublicationCopies(t *testing.T) {
	backend := backendResource("orders")
	backend.Spec.TLS = &gatewayv1alpha1.BackendTLSSettings{SNI: "orders.example.com", WellKnownCACertificates: "System"}
	compiled := compileNetworkBackend(&backend)
	want := compileNetworkBackend(&backend)

	http := compiled.httpBackend()
	ai := compiled.aiBackend()
	http.Endpoints[0].Address = "http.example.com"
	http.TLS.SNI = "http.example.com"
	ai.Endpoints[0].Address = "ai.example.com"
	ai.TLS.SNI = "ai.example.com"
	ai.TLS.WellKnownCACertificates = "changed"

	if diff := cmp.Diff(want, compiled, cmp.AllowUnexported(networkBackend{})); diff != "" {
		t.Errorf("publication modified cached network result (-want +got):\n%s", diff)
	}
}

func TestBackendSourceSuppressesEquivalentNetworkChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		producer := informertesting.NewFakeControllerSource()
		t.Cleanup(producer.Shutdown)
		backend := backendResource("orders")
		backend.Spec.Endpoints = append(backend.Spec.Endpoints, gatewayv1alpha1.Endpoint{Address: "other.example.com", Port: 8082, Weight: 2})
		producer.Add(backend.DeepCopy())
		informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Backend{}, 0, nil)
		s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Backend{}, compileNetworkBackend)
		queue := newResourceSourceQueue(t)
		if err := s.Start(t.Context(), queue); err != nil {
			t.Fatalf("Start() error = %v, want nil", err)
		}

		runResourceInformer(t, informer)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		if err := s.WaitForSync(ctx); err != nil {
			t.Fatalf("WaitForSync() error = %v, want nil", err)
		}

		if got := queue.Len(); got != 1 {
			t.Fatalf("initial queue length = %d, want 1", got)
		}

		request, _ := queue.Get()
		queue.Done(request)
		previous := *s.projected.GetKey("orders")
		backend.Generation++
		backend.ResourceVersion = "2"
		backend.Spec.DisplayName = "新的展示名称"
		backend.Status.Conditions = []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}}
		slices.Reverse(backend.Spec.Endpoints)

		producer.Modify(backend.DeepCopy())
		synctest.Wait()

		if got := queue.Len(); got != 0 {
			t.Errorf("equivalent network change queue length = %d, want 0", got)
		}

		if got := s.projected.GetKey("orders"); got == nil || *got != previous {
			t.Error("equivalent network change replaced cached result, want same result instance")
		}

		backend.Spec.Endpoints[0].Weight++
		producer.Modify(backend.DeepCopy())
		synctest.Wait()

		if got := queue.Len(); got != 1 {
			t.Errorf("endpoint weight change queue length = %d, want 1", got)
		}
	})
}

func TestReconcileReusesDerivedBackends(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		snapshot := aiResourceSnapshot()
		route := routeResource("ordinary", 1, "public", "api", "model-network", gatewayv1alpha1.PathMatchPrefix)
		route.Namespace = "team-a"
		snapshot.httpRoutes.Items = append(snapshot.httpRoutes.Items, route)
		apiClient := aiResourceClient(t, snapshot)
		reads := interceptor.NewClient(apiClient, interceptor.Funcs{
			List: func(ctx context.Context, underlying client.WithWatch, list client.ObjectList, options ...client.ListOption) error {
				if _, backendList := list.(*gatewayv1alpha1.BackendList); backendList {
					t.Fatal("Reconcile read raw Backends instead of derived network results")
				}

				return underlying.List(ctx, list, options...)
			},
		})
		producer := informertesting.NewFakeControllerSource()
		t.Cleanup(producer.Shutdown)
		for i := range snapshot.backends.Items {
			producer.Add(snapshot.backends.Items[i].DeepCopy())
		}

		var modelCalls, toolsCalls atomic.Int64
		informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Backend{}, 0, nil)
		s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Backend{}, func(backend *gatewayv1alpha1.Backend) *networkBackend {
			if backend.Name == "model-network" {
				modelCalls.Add(1)
			} else {
				toolsCalls.Add(1)
			}

			return compileNetworkBackend(backend)
		})
		if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
			t.Fatalf("Start() error = %v, want nil", err)
		}

		runResourceInformer(t, informer)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		if err := s.WaitForSync(ctx); err != nil {
			t.Fatalf("WaitForSync() error = %v, want nil", err)
		}

		var http dataplane.Config
		var ai aidataplane.Config
		publisher := aiPublisherStub{
			publisherFunc: func(_ context.Context, config dataplane.Config) error { http = config; return nil },
			publishAI:     func(_ context.Context, config aidataplane.Config) error { ai = config; return nil },
		}
		r := New(reads, apiClient, publisher, "control-plane")
		setTestResources(t, r, snapshot)
		r.sources.backends = s
		r.sources.registerBindings(t.Context(), newResourceSourceQueue(t))
		synctest.Wait()
		for range 2 {
			if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
				t.Fatalf("Reconcile() error = %v, want nil", err)
			}
		}

		if modelCalls.Load() != 1 || toolsCalls.Load() != 1 {
			t.Fatalf("initial network conversions = %d/%d, want 1/1", modelCalls.Load(), toolsCalls.Load())
		}

		if err := apiClient.Get(t.Context(), client.ObjectKeyFromObject(&route), &route); err != nil {
			t.Fatalf("Get(HTTPRoute) error = %v, want nil", err)
		}

		route.Spec.Rules[0].Matches[0].Path.Value = "/v2/orders"
		if err := apiClient.Update(t.Context(), &route); err != nil {
			t.Fatalf("Update(HTTPRoute) error = %v, want nil", err)
		}

		// 这个测试单独驱动 Backend Informer，Route 在集合边界提供更新后的匹配结果。
		r.sources.httpRoutes.projected = krt.NewStaticCollection(nil, []*httpRoute{compileHTTPRoute(&route)}, krt.WithStop(t.Context().Done()))

		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(changed Route) error = %v, want nil", err)
		}

		if modelCalls.Load() != 1 || toolsCalls.Load() != 1 {
			t.Errorf("network conversions after Route update = %d/%d, want 1/1", modelCalls.Load(), toolsCalls.Load())
		}

		backend := snapshot.backends.Items[0].DeepCopy()
		backend.Spec.Endpoints[0].Address = "new-model.example.com"
		backend.Generation++
		producer.Modify(backend.DeepCopy())
		synctest.Wait()

		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(changed Backend) error = %v, want nil", err)
		}

		if modelCalls.Load() != 2 || toolsCalls.Load() != 1 {
			t.Errorf("network conversions after Backend update = %d/%d, want 2/1", modelCalls.Load(), toolsCalls.Load())
		}

		if len(http.Backends) != 1 || http.Backends[0].Endpoints[0].Address != "new-model.example.com" {
			t.Errorf("HTTP Backends = %+v, want updated model-network", http.Backends)
		}

		if len(ai.NetworkBackends) != 2 || ai.NetworkBackends[0].Endpoints[0].Address != "new-model.example.com" {
			t.Errorf("AI network backends = %+v, want updated model-network and unchanged tools-network", ai.NetworkBackends)
		}

		backend.DeletionTimestamp = new(metav1.Unix(3, 0))
		producer.Modify(backend.DeepCopy())
		synctest.Wait()
		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(deleting Backend) error = %v, want nil", err)
		}

		// 保留现有区别：HTTP 到实际删除时撤销；AI 在删除开始时即标为不可用。
		if len(http.Backends) != 1 || len(ai.NetworkBackends) != 1 || *ai.Listeners[0].Hosts[0].ModelRoutes[0].BackendName != "" {
			t.Errorf("deleting Backend HTTP/AI results = %+v/%+v, want retained HTTP and unavailable model", http.Backends, ai.NetworkBackends)
		}

		producer.Delete(backend.DeepCopy())
		synctest.Wait()
		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(deleted Backend) error = %v, want nil", err)
		}

		if len(http.Backends) != 0 || http.Listeners[0].Hosts[0].Routes[0].BackendName != "" {
			t.Errorf("deleted Backend HTTP results = %+v, want unavailable match without network output", http)
		}

		backend.DeletionTimestamp = nil
		producer.Add(backend.DeepCopy())
		synctest.Wait()
		if _, err := r.Reconcile(t.Context(), sharedRequest()); err != nil {
			t.Fatalf("Reconcile(restored Backend) error = %v, want nil", err)
		}

		if len(http.Backends) != 1 || len(ai.NetworkBackends) != 2 || *ai.Listeners[0].Hosts[0].ModelRoutes[0].BackendName == "" {
			t.Errorf("restored Backend HTTP/AI results = %+v/%+v, want restored model-network", http.Backends, ai.NetworkBackends)
		}

		if got := toolsCalls.Load(); got != 1 {
			t.Errorf("unrelated tools-network conversions = %d, want 1", got)
		}
	})
}
