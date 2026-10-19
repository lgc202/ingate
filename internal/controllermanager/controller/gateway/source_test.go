package gateway

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"istio.io/istio/pkg/kube/krt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	toolscache "k8s.io/client-go/tools/cache"
	informertesting "k8s.io/client-go/tools/cache/testing"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

type resourceSourceCache struct {
	cache.Cache
	informer cache.Informer
}

type replayBlockedInformer struct {
	cache.Informer
	entered chan struct{}
	release <-chan struct{}
	stop    <-chan struct{}
}

type registrationFailedInformer struct {
	cache.Informer
	err error
}

type derivedGateway struct {
	key        string
	generation int64
}

func TestResourceSourceInitialReplay(t *testing.T) {
	tests := []struct {
		name    string
		objects []*gatewayv1alpha1.Gateway
	}{
		{name: "empty"},
		{
			name: "existing object",
			objects: []*gatewayv1alpha1.Gateway{{
				ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			producer := informertesting.NewFakeControllerSource()
			t.Cleanup(producer.Shutdown)
			for _, object := range tt.objects {
				producer.Add(object)
			}

			informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Gateway{}, 0, nil)
			s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Gateway{}, projectGateway)
			queue := newResourceSourceQueue(t)
			if err := s.Start(t.Context(), queue); err != nil {
				t.Fatalf("Start() error = %v, want nil", err)
			}

			if s.objects.HasSynced() {
				t.Error("objects.HasSynced() before replay = true, want false")
			}

			runResourceInformer(t, informer)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			if err := s.WaitForSync(ctx); err != nil {
				t.Fatalf("WaitForSync() error = %v, want nil", err)
			}

			if got := len(s.objects.List()); got != len(tt.objects) {
				t.Errorf("objects.List() length = %d, want %d", got, len(tt.objects))
			}

			if got := len(s.projected.List()); got != len(tt.objects) {
				t.Errorf("projected.List() length = %d, want %d", got, len(tt.objects))
			}

			if got := queue.Len(); got != len(tt.objects) {
				t.Errorf("initial queue length = %d, want %d", got, len(tt.objects))
			}
		})
	}
}

func TestResourceSourceWaitsForItsHandler(t *testing.T) {
	producer := informertesting.NewFakeControllerSource()
	t.Cleanup(producer.Shutdown)
	producer.Add(&gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "edge"}})

	informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Gateway{}, 0, nil)
	release := make(chan struct{})
	blocked := &replayBlockedInformer{
		Informer: informer,
		entered:  make(chan struct{}),
		release:  release,
		stop:     t.Context().Done(),
	}
	s := newResourceSource(&resourceSourceCache{informer: blocked}, &gatewayv1alpha1.Gateway{}, projectGateway)
	if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	runResourceInformer(t, informer)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	select {
	case <-blocked.entered:
	case <-ctx.Done():
		t.Fatal("initial handler was not called")
	}

	if !toolscache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		t.Fatal("Informer store did not sync")
	}

	if s.objects.HasSynced() {
		t.Error("objects.HasSynced() while handler is blocked = true, want false")
	}

	canceled, stop := context.WithCancel(ctx)
	stop()
	if err := s.WaitForSync(canceled); !errors.Is(err, context.Canceled) {
		t.Errorf("WaitForSync(blocked handler) error = %v, want context.Canceled", err)
	}

	if s.objects.HasSynced() {
		t.Error("objects.HasSynced() after canceled wait = true, want false")
	}

	close(release)
}

func TestResourceSourceReturnsRegistrationFailure(t *testing.T) {
	cause := errors.New("handler registration unavailable")
	s := newResourceSource(&resourceSourceCache{informer: &registrationFailedInformer{err: cause}}, &gatewayv1alpha1.Gateway{}, projectGateway)
	if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
		t.Fatalf("Start() error = %v, want asynchronous initialization", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := s.WaitForSync(ctx)
	if !errors.Is(err, cause) {
		t.Errorf("WaitForSync() error = %v, want wrapped %v", err, cause)
	}
}

func TestResourceSourceNotifiesRuntimeChanges(t *testing.T) {
	tests := []struct {
		name   string
		change func(*gatewayv1alpha1.Gateway)
		queued bool
	}{
		{name: "resource version", change: func(resource *gatewayv1alpha1.Gateway) { resource.ResourceVersion = "2" }},
		{name: "status", change: func(resource *gatewayv1alpha1.Gateway) {
			resource.Status.Conditions = []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}}
		}},
		{name: "labels", change: func(resource *gatewayv1alpha1.Gateway) { resource.Labels = map[string]string{"team": "a"} }},
		{name: "annotations", change: func(resource *gatewayv1alpha1.Gateway) { resource.Annotations = map[string]string{"note": "updated"} }},
		{name: "finalizers", change: func(resource *gatewayv1alpha1.Gateway) { resource.Finalizers = []string{"example.com/cleanup"} }},
		{name: "display name and generation", change: func(resource *gatewayv1alpha1.Gateway) {
			resource.Spec.DisplayName = "新的展示名称"
			resource.Generation++
		}, queued: true},
		{name: "listener port", change: func(resource *gatewayv1alpha1.Gateway) {
			resource.Spec.Listeners[0].Port = 9090
			resource.Generation++
		}, queued: true},
		{name: "deletion starts", change: func(resource *gatewayv1alpha1.Gateway) {
			resource.DeletionTimestamp = new(metav1.Unix(3, 0))
		}, queued: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				producer := informertesting.NewFakeControllerSource()
				t.Cleanup(producer.Shutdown)
				resource := &gatewayv1alpha1.Gateway{
					ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge", Generation: 1},
					Spec: gatewayv1alpha1.GatewaySpec{
						Listeners: []gatewayv1alpha1.Listener{{Name: "http", Protocol: gatewayv1alpha1.ProtocolHTTP, Port: 8080}},
					},
				}
				producer.Add(resource.DeepCopy())
				informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Gateway{}, 0, nil)
				s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Gateway{}, projectGateway)
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
				tt.change(resource)

				producer.Modify(resource.DeepCopy())
				synctest.Wait()

				if got := queue.Len() != 0; got != tt.queued {
					t.Errorf("resource update (%s) queued = %t, want %t", tt.name, got, tt.queued)
				}
			})
		})
	}
}

// 验证适配后的输入可直接用于按对象派生，A 的更新和删除不重算同名的 B。
func TestResourceSourceSupportsPerObjectDerivation(t *testing.T) {
	producer := informertesting.NewFakeControllerSource()
	t.Cleanup(producer.Shutdown)
	first := &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge", Generation: 1}}
	second := &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: "team-b", Name: "edge", Generation: 1}}
	producer.Add(first.DeepCopy())
	producer.Add(second.DeepCopy())

	informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Gateway{}, 0, nil)
	s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Gateway{}, projectGateway)
	if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	var firstCalls, secondCalls atomic.Int64
	derived := krt.NewCollection(s.projected, func(_ krt.HandlerContext, gateway *gatewayv1alpha1.Gateway) *derivedGateway {
		if gateway.Namespace == "team-a" {
			firstCalls.Add(1)
		} else {
			secondCalls.Add(1)
		}

		return &derivedGateway{key: krt.GetKey(gateway), generation: gateway.Generation}
	}, krt.WithStop(t.Context().Done()))

	runResourceInformer(t, informer)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if err := s.WaitForSync(ctx); err != nil {
		t.Fatalf("WaitForSync() error = %v, want nil", err)
	}

	if !derived.WaitUntilSynced(ctx.Done()) {
		t.Fatal("derived collection did not sync")
	}

	previousFirstCalls := firstCalls.Load()
	previousSecondCalls := secondCalls.Load()
	first.Generation++
	producer.Modify(first.DeepCopy())
	if err := wait.PollUntilContextCancel(ctx, time.Millisecond, true, func(context.Context) (bool, error) {
		current := derived.GetKey("team-a/edge")

		return current != nil && current.generation == 2, nil
	}); err != nil {
		t.Fatalf("collection did not receive team-a update: %v", err)
	}

	producer.Delete(first.DeepCopy())
	if err := wait.PollUntilContextCancel(ctx, time.Millisecond, true, func(context.Context) (bool, error) {
		return derived.GetKey("team-a/edge") == nil, nil
	}); err != nil {
		t.Fatalf("collection did not receive team-a deletion: %v", err)
	}

	if current := derived.GetKey("team-b/edge"); current == nil || current.generation != 1 {
		t.Errorf("collection team-b/edge = %v, want unchanged generation 1", current)
	}

	if got := firstCalls.Load(); got <= previousFirstCalls {
		t.Errorf("team-a transform calls = %d, want more than %d", got, previousFirstCalls)
	}

	if got := secondCalls.Load(); got != previousSecondCalls {
		t.Errorf("team-b transform calls = %d, want unchanged %d", got, previousSecondCalls)
	}
}

func (c *resourceSourceCache) GetInformer(
	_ context.Context,
	_ client.Object,
	_ ...cache.InformerGetOption,
) (cache.Informer, error) {
	return c.informer, nil
}

func (c *resourceSourceCache) WaitForCacheSync(ctx context.Context) bool {
	return toolscache.WaitForCacheSync(ctx.Done(), c.informer.HasSynced)
}

func (i *replayBlockedInformer) AddEventHandlerWithOptions(
	handler toolscache.ResourceEventHandler,
	options toolscache.HandlerOptions,
) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.Informer.AddEventHandlerWithOptions(toolscache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(object any, initial bool) {
			close(i.entered)
			select {
			case <-i.release:
				handler.OnAdd(object, initial)
			case <-i.stop:
			}
		},
		UpdateFunc: handler.OnUpdate,
		DeleteFunc: handler.OnDelete,
	}, options)
}

func (i *registrationFailedInformer) AddEventHandlerWithOptions(
	_ toolscache.ResourceEventHandler,
	_ toolscache.HandlerOptions,
) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, i.err
}

func (g derivedGateway) ResourceName() string {
	return g.key
}

func newResourceSourceQueue(t *testing.T) workqueue.TypedRateLimitingInterface[reconcile.Request] {
	t.Helper()
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	t.Cleanup(queue.ShutDown)

	return queue
}

func runResourceInformer(t *testing.T, informer toolscache.SharedIndexInformer) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		informer.RunWithContext(t.Context())
	}()
	t.Cleanup(func() {
		<-done
	})
}
