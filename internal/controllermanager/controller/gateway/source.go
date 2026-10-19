package gateway

import (
	"context"
	"fmt"

	"istio.io/istio/pkg/kube/controllers"
	"istio.io/istio/pkg/kube/krt"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// resourceSource 将 Manager 的 Informer 事件接入 krt，不创建第二套 List/Watch。
// 集合中的对象沿用 Informer 的只读契约，后续转换不得原地修改对象。
type resourceSource[T client.Object] struct {
	cache        cache.Cache
	object       T
	objects      krt.StaticCollection[T]
	kind         source.SyncingSource
	subscription krt.HandlerRegistration
	replayed     chan struct{}
}

// replaySync 在本次注册的初始事件处理完成后，才允许派生集合报告已同步。
// Informer.HasSynced 只说明底层 Store 已同步，不保证这个 Handler 已收到全部初始对象。
type replaySync struct {
	done <-chan struct{}
}

var _ source.SyncingSource = (*resourceSource[*gatewayv1alpha1.Gateway])(nil)

func newResourceSource[T client.Object](resourceCache cache.Cache, object T) *resourceSource[T] {
	return &resourceSource[T]{cache: resourceCache, object: object}
}

func (s *resourceSource[T]) Start(
	ctx context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	s.replayed = make(chan struct{})
	s.objects = krt.NewMutableCollection[T](
		replaySync{done: s.replayed},
		nil,
		krt.WithStop(ctx.Done()),
		krt.WithName(fmt.Sprintf("gateway/%T", s.object)),
	)
	s.subscription = s.objects.AsCollection().RegisterBatch(func(changes []krt.Event[T]) {
		enqueueResourceChanges(queue, changes)
	}, true)

	// 原生 Kind 负责异步取得 Informer、启动重试和 Handler 初始回放等待。
	// Manager 停止 Informer；krt 使用相同的运行 Context，不另建运行循环。
	s.kind = source.Kind(s.cache, s.object, handler.TypedFuncs[T, reconcile.Request]{
		CreateFunc: func(
			_ context.Context,
			created event.TypedCreateEvent[T],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			s.objects.UpdateObject(created.Object)
		},
		UpdateFunc: func(
			_ context.Context,
			updated event.TypedUpdateEvent[T],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			s.objects.UpdateObject(updated.ObjectNew)
		},
		DeleteFunc: func(
			_ context.Context,
			deleted event.TypedDeleteEvent[T],
			_ workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			s.objects.DeleteObject(krt.GetKey(deleted.Object))
		},
	})

	return s.kind.Start(ctx, queue)
}

func (s *resourceSource[T]) WaitForSync(ctx context.Context) error {
	if err := s.kind.WaitForSync(ctx); err != nil {
		return fmt.Errorf("sync %T source: %w", s.object, err)
	}

	// Kind 在等待被取消时可以返回 nil，不能据此把尚未回放的空集合标为已同步。
	if err := ctx.Err(); err != nil {
		return err
	}

	close(s.replayed)
	if !s.subscription.WaitUntilSynced(ctx.Done()) {
		return fmt.Errorf("wait for %T collection handler: %w", s.object, ctx.Err())
	}

	return nil
}

func (s replaySync) HasSynced() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s replaySync) WaitUntilSynced(stop <-chan struct{}) bool {
	select {
	case <-s.done:
		return true
	case <-stop:
		return false
	}
}

func enqueueResourceChanges[T client.Object](
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
	changes []krt.Event[T],
) {
	for _, change := range changes {
		if change.Event == controllers.EventUpdate && !resourcePredicate().Update(event.UpdateEvent{
			ObjectOld: *change.Old,
			ObjectNew: *change.New,
		}) {
			continue
		}

		queue.Add(sharedRequest())
	}
}
