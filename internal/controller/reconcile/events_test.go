package reconcile

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func TestEventHandlerUsesSingleConfigKey(t *testing.T) {
	oldResource := &gatewayv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "gateway-a", Generation: 1}}
	newResource := oldResource.DeepCopy()
	newResource.Generation = 2

	tests := []struct {
		name    string
		handle  func(cache.ResourceEventHandlerFuncs)
		enqueue bool
	}{
		{
			name: "add",
			handle: func(handler cache.ResourceEventHandlerFuncs) {
				handler.AddFunc(newResource)
			},
			enqueue: true,
		},
		{
			name: "delete",
			handle: func(handler cache.ResourceEventHandlerFuncs) {
				handler.DeleteFunc(cache.DeletedFinalStateUnknown{Key: newResource.Name, Obj: newResource})
			},
			enqueue: true,
		},
		{
			name: "spec update",
			handle: func(handler cache.ResourceEventHandlerFuncs) {
				handler.UpdateFunc(oldResource, newResource)
			},
			enqueue: true,
		},
		{
			name: "status update",
			handle: func(handler cache.ResourceEventHandlerFuncs) {
				statusOnly := oldResource.DeepCopy()
				statusOnly.ResourceVersion = "2"
				handler.UpdateFunc(oldResource, statusOnly)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			queue := workqueue.NewTypedRateLimitingQueueWithConfig(
				workqueue.DefaultTypedControllerRateLimiter[string](),
				workqueue.TypedRateLimitingQueueConfig[string]{Name: "events-test"},
			)
			t.Cleanup(queue.ShutDown)
			r := &Reconciler{queue: queue}

			test.handle(r.eventHandler())
			if !test.enqueue {
				if got := queue.Len(); got != 0 {
					t.Errorf("queue.Len() = %d, want 0", got)
				}
				return
			}
			if got, want := queue.Len(), 1; got != want {
				t.Fatalf("queue.Len() = %d, want %d", got, want)
			}
			key, shutdown := queue.Get()
			if shutdown {
				t.Fatal("queue.Get() shutdown = true, want false")
			}
			queue.Done(key)
			queue.Forget(key)
			if got, want := key, queueKey; got != want {
				t.Errorf("queue key = %q, want %q", got, want)
			}
		})
	}
}
