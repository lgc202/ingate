// Package gateway 协调声明式网关资源，共同生成一份共享数据面配置。
package gateway

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

// sharedRequestName 是所有资源事件共用的队列键，不对应真实 API 对象。
const sharedRequestName = "shared-data-plane"

// Publisher 一次替换完整配置；成功不代表 Envoy 已接收或 ACK。
type Publisher interface {
	Publish(context.Context, dataplane.Config) error
}

// Reconciler 协调全部 Gateway、HTTPRoute 和 Backend，共同更新共享数据面。
type Reconciler struct {
	client    client.Client
	publisher Publisher
}

// New 创建 Reconciler；Controller 的运行生命周期由 Manager 管理。
func New(apiClient client.Client, publisher Publisher) *Reconciler {
	return &Reconciler{client: apiClient, publisher: publisher}
}

// SetupWithManager 将三种资源事件映射到同一个队列键，并在启动时主动协调一次。
func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("gateway").
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Watches(
			&gatewayv1alpha1.Gateway{},
			handler.EnqueueRequestsFromMapFunc(sharedRequests),
		).
		Watches(
			&gatewayv1alpha1.HTTPRoute{},
			handler.EnqueueRequestsFromMapFunc(sharedRequests),
		).
		Watches(
			&gatewayv1alpha1.Backend{},
			handler.EnqueueRequestsFromMapFunc(sharedRequests),
		).
		WatchesRawSource(source.Func(enqueueInitialReconcile)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 从当前 Cache 重建并发布完整配置，再写回资源的接纳状态。
// 请求键只负责合并事件。
func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	var gateways gatewayv1alpha1.GatewayList
	if err := r.client.List(ctx, &gateways); err != nil {
		return ctrl.Result{}, fmt.Errorf("list Gateways: %w", err)
	}

	var routes gatewayv1alpha1.HTTPRouteList
	if err := r.client.List(ctx, &routes); err != nil {
		return ctrl.Result{}, fmt.Errorf("list HTTPRoutes: %w", err)
	}

	var backends gatewayv1alpha1.BackendList
	if err := r.client.List(ctx, &backends); err != nil {
		return ctrl.Result{}, fmt.Errorf("list Backends: %w", err)
	}

	result := resolve(gateways.Items, routes.Items, backends.Items)
	config := compile(result)

	if err := r.publisher.Publish(ctx, config); err != nil {
		return ctrl.Result{}, fmt.Errorf("publish data plane configuration: %w", err)
	}

	if err := r.updateGatewayStatuses(ctx, gateways.Items, result.gatewayConflicts); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateHTTPRouteStatuses(ctx, routes.Items, result.parents); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func sharedRequests(_ context.Context, _ client.Object) []reconcile.Request {
	return []reconcile.Request{sharedRequest()}
}

// enqueueInitialReconcile 保证空资源集合或 Controller 重启后也会发布一次。
func enqueueInitialReconcile(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(sharedRequest())
	return nil
}

func sharedRequest() reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: sharedRequestName}}
}
