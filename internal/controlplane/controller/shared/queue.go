package shared

import (
	"k8s.io/client-go/util/workqueue"
)

type GatewayQueue struct {
	queue workqueue.TypedRateLimitingInterface[ObjectKey]
}

func NewGatewayQueue() *GatewayQueue {
	return &GatewayQueue{
		queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[ObjectKey]()),
	}
}

func (q *GatewayQueue) Enqueue(key ObjectKey) {
	if q == nil || q.queue == nil {
		return
	}
	if key.Name == "" {
		return
	}
	q.queue.Add(key)
}

func (q *GatewayQueue) Requeue(key ObjectKey) {
	if q == nil || q.queue == nil {
		return
	}
	if key.Name == "" {
		return
	}
	q.queue.AddRateLimited(key)
}

func (q *GatewayQueue) Get() (ObjectKey, bool) {
	if q == nil || q.queue == nil {
		return ObjectKey{}, true
	}

	item, shutdown := q.queue.Get()
	if shutdown {
		return ObjectKey{}, true
	}
	return item, false
}

func (q *GatewayQueue) Done(key ObjectKey) {
	if q == nil || q.queue == nil || key.Name == "" {
		return
	}
	q.queue.Done(key)
}

func (q *GatewayQueue) Forget(key ObjectKey) {
	if q == nil || q.queue == nil || key.Name == "" {
		return
	}
	q.queue.Forget(key)
}

func (q *GatewayQueue) ShutDown() {
	if q == nil || q.queue == nil {
		return
	}
	q.queue.ShutDown()
}
