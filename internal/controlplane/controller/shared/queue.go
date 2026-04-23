package shared

import (
	"k8s.io/client-go/util/workqueue"
)

type GatewayQueue interface {
	Enqueue(ObjectKey)
	Requeue(ObjectKey)
	Get() (ObjectKey, bool)
	Done(ObjectKey)
	Forget(ObjectKey)
	ShutDown()
}

type gatewayQueue struct {
	queue workqueue.TypedRateLimitingInterface[ObjectKey]
}

func NewGatewayQueue() GatewayQueue {
	return &gatewayQueue{
		queue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[ObjectKey]()),
	}
}

func (q *gatewayQueue) Enqueue(key ObjectKey) {
	if q == nil || q.queue == nil {
		return
	}
	if key.Name == "" {
		return
	}
	q.queue.Add(key)
}

func (q *gatewayQueue) Requeue(key ObjectKey) {
	if q == nil || q.queue == nil {
		return
	}
	if key.Name == "" {
		return
	}
	q.queue.AddRateLimited(key)
}

func (q *gatewayQueue) Get() (ObjectKey, bool) {
	if q == nil || q.queue == nil {
		return ObjectKey{}, true
	}

	item, shutdown := q.queue.Get()
	if shutdown {
		return ObjectKey{}, true
	}
	return item, false
}

func (q *gatewayQueue) Done(key ObjectKey) {
	if q == nil || q.queue == nil || key.Name == "" {
		return
	}
	q.queue.Done(key)
}

func (q *gatewayQueue) Forget(key ObjectKey) {
	if q == nil || q.queue == nil || key.Name == "" {
		return
	}
	q.queue.Forget(key)
}

func (q *gatewayQueue) ShutDown() {
	if q == nil || q.queue == nil {
		return
	}
	q.queue.ShutDown()
}
