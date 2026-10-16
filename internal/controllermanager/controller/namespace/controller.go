// Package namespace 使用 Manager 的事件和工作队列完成 Namespace 内容清理及 finalize。
package namespace

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/lgc202/ingate/internal/controllermanager/controller/namespace/deletion"
)

const (
	controllerName = "namespace"

	// namespaceDeletionGracePeriod 沿用官方删除缓冲，给准入缓存观察 Terminating 留出时间。
	// 它不保证跨资源原子性，也不能替代准入或资源残留检查。
	namespaceDeletionGracePeriod = 5 * time.Second
)

// Reconciler 根据正在删除的 Namespace 协调其内容；不自建 Informer、队列或协程。
type Reconciler struct {
	reader  client.Reader
	deleter *deletion.Deleter
}

// New 保存 Manager 的读取入口和删除内核，不读取 API 或启动工作任务。
func New(reader client.Reader, deleter *deletion.Deleter) *Reconciler {
	return &Reconciler{reader: reader, deleter: deleter}
}

// SetupWithManager 注册 Namespace Watch；缓存同步、同键去重和退出由 Manager 负责。
func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named(controllerName).
		For(&corev1.Namespace{}).
		WithEventFilter(namespaceEvents()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 忽略已消失、未删除及已完成的空间；内容等待定时重排，操作失败返回框架重试。
func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	namespace := &corev1.Namespace{}
	if err := r.reader.Get(ctx, request.NamespacedName, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get namespace %q from cache: %w", request.Name, err)
	}

	if namespace.DeletionTimestamp.IsZero() || len(namespace.Spec.Finalizers) == 0 {
		return ctrl.Result{}, nil
	}

	// 缓冲按 API 记录的删除时间计算，不在 worker 中 sleep，也不因每次 status 写回重新计时。
	if delay := time.Until(namespace.DeletionTimestamp.Add(namespaceDeletionGracePeriod)); delay > 0 {
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	err := r.deleter.Delete(ctx, namespace.Name)
	if err == nil {
		return ctrl.Result{}, nil
	}

	if remaining, ok := errors.AsType[*deletion.ResourcesRemainingError](err); ok {
		// 沿用官方 Estimate/2+1 的检查间隔；正常等待不进入失败退避或重复错误日志。
		delay := time.Duration(remaining.Estimate/2+1) * time.Second
		return ctrl.Result{RequeueAfter: delay}, nil
	}

	return ctrl.Result{}, err
}

// namespaceEvents 接纳删除启动、对象重建和 finalizer 变化；status 写回不形成自触发循环。
func namespaceEvents() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return !e.Object.GetDeletionTimestamp().IsZero()
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			previous, previousOK := e.ObjectOld.(*corev1.Namespace)
			current, currentOK := e.ObjectNew.(*corev1.Namespace)
			if !previousOK || !currentOK || current.DeletionTimestamp.IsZero() {
				return false
			}

			return previous.UID != current.UID ||
				!previous.DeletionTimestamp.Equal(current.DeletionTimestamp) ||
				!slices.Equal(previous.Spec.Finalizers, current.Spec.Finalizers) ||
				!slices.Equal(previous.Finalizers, current.Finalizers)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
