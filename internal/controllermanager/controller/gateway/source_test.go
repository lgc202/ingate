package gateway

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"istio.io/istio/pkg/kube/controllers"
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
			s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Gateway{})
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
	s := newResourceSource(&resourceSourceCache{informer: blocked}, &gatewayv1alpha1.Gateway{})
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
	s := newResourceSource(&resourceSourceCache{informer: &registrationFailedInformer{err: cause}}, &gatewayv1alpha1.Gateway{})
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

// 验证适配后的输入可直接用于按对象派生，A 的更新和删除不重算同名的 B。
func TestResourceSourceSupportsPerObjectDerivation(t *testing.T) {
	producer := informertesting.NewFakeControllerSource()
	t.Cleanup(producer.Shutdown)
	first := &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "edge", Generation: 1}}
	second := &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Namespace: "team-b", Name: "edge", Generation: 1}}
	producer.Add(first.DeepCopy())
	producer.Add(second.DeepCopy())

	informer := toolscache.NewSharedIndexInformer(producer, &gatewayv1alpha1.Gateway{}, 0, nil)
	s := newResourceSource(&resourceSourceCache{informer: informer}, &gatewayv1alpha1.Gateway{})
	if err := s.Start(t.Context(), newResourceSourceQueue(t)); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}

	var firstCalls, secondCalls atomic.Int64
	derived := krt.NewCollection(s.objects.AsCollection(), func(_ krt.HandlerContext, gateway *gatewayv1alpha1.Gateway) *derivedGateway {
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

func TestEnqueueResourceChanges(t *testing.T) {
	previous := &gatewayv1alpha1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "edge", Generation: 1}}
	statusOnly := previous.DeepCopy()
	statusOnly.Status.Conditions = []metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}}
	specChange := previous.DeepCopy()
	specChange.Generation++
	deleting := previous.DeepCopy()
	deleting.DeletionTimestamp = new(metav1.Now())

	tests := []struct {
		name   string
		change krt.Event[*gatewayv1alpha1.Gateway]
		queued bool
	}{
		{
			name:   "create",
			change: krt.Event[*gatewayv1alpha1.Gateway]{Event: controllers.EventAdd, New: new(previous)},
			queued: true,
		},
		{
			name:   "status",
			change: krt.Event[*gatewayv1alpha1.Gateway]{Event: controllers.EventUpdate, Old: new(previous), New: new(statusOnly)},
		},
		{
			name:   "spec",
			change: krt.Event[*gatewayv1alpha1.Gateway]{Event: controllers.EventUpdate, Old: new(previous), New: new(specChange)},
			queued: true,
		},
		{
			name:   "deletion begins",
			change: krt.Event[*gatewayv1alpha1.Gateway]{Event: controllers.EventUpdate, Old: new(previous), New: new(deleting)},
			queued: true,
		},
		{
			name:   "delete",
			change: krt.Event[*gatewayv1alpha1.Gateway]{Event: controllers.EventDelete, Old: new(previous)},
			queued: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := newResourceSourceQueue(t)

			enqueueResourceChanges(queue, []krt.Event[*gatewayv1alpha1.Gateway]{tt.change})

			if got := queue.Len() != 0; got != tt.queued {
				t.Errorf("enqueueResourceChanges(%s) queued = %t, want %t", tt.name, got, tt.queued)
			}
		})
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
