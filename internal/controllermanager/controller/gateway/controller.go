// Package gateway 按资源作用域解析引用，编译共享 HTTP 与 AI 数据面配置。
package gateway

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/lgc202/ingate/internal/controllermanager/dataplane"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const (
	// sharedRequestName 是所有资源事件共用的队列键，不对应真实 API 对象。
	sharedRequestName = "shared-data-plane"
	// controllerName 标识本控制器拥有的父引用状态。
	controllerName = "gateway.ingate.io/gateway-controller"
)

// Publisher 分别替换 HTTP 与 AI 期望配置，在适配器内合成完整 xDS 视图。
// 构建失败保留该路最近有效的结果；通知失败可能已更新缓存，必须重试而非假定回滚。
// 成功不代表 Envoy 已接收或 ACK。
type Publisher interface {
	Publish(context.Context, dataplane.Config) error
	PublishAI(context.Context, aidataplane.Config) error
}

// Reconciler 协调六种领域资源，分别发布共享 HTTP 与 AI 配置。
type Reconciler struct {
	client    client.Client
	publisher Publisher
	aiConfig  aiConfigWriter
}

// New 创建 Reconciler；Controller 的运行生命周期由 Manager 管理。
// apiReader 使用 Manager 的 APIReader，namespace 是预先创建的控制面空间。
func New(
	apiClient client.Client,
	apiReader client.Reader,
	publisher Publisher,
	namespace string,
) *Reconciler {
	return &Reconciler{
		client:    apiClient,
		publisher: publisher,
		aiConfig:  aiConfigWriter{client: apiClient, reader: apiReader, namespace: namespace},
	}
}

// SetupWithManager 通过 krt 运行投影合并六类领域资源事件，受管 ConfigMap 保留原生 Watch。
// 所有源完成初始同步后才启动 Worker；空资源集合也主动协调一次。
func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("gateway").
		WatchesRawSource(newResourceSource(manager.GetCache(), &gatewayv1alpha1.Gateway{}, projectGateway)).
		WatchesRawSource(newResourceSource(manager.GetCache(), &gatewayv1alpha1.HTTPRoute{}, projectHTTPRoute)).
		WatchesRawSource(newResourceSource(manager.GetCache(), &gatewayv1alpha1.Backend{}, projectBackend)).
		WatchesRawSource(newResourceSource(manager.GetCache(), &gatewayv1alpha1.AIBackend{}, projectAIBackend)).
		WatchesRawSource(newResourceSource(manager.GetCache(), &gatewayv1alpha1.ModelRoute{}, projectModelRoute)).
		WatchesRawSource(newResourceSource(manager.GetCache(), &gatewayv1alpha1.MCPRoute{}, projectMCPRoute)).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(sharedRequests),
			builder.WithPredicates(r.aiConfig.predicate()),
		).
		WatchesRawSource(source.Func(enqueueInitialReconcile)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 从当前 Cache 重建两份完整配置；请求键只负责合并事件。
// 两路输出不是事务，一路失败不阻断另一路；错误交给工作队列限速重试。
func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	snapshot, err := r.readResources(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	httpErr := r.reconcileHTTP(ctx, snapshot)
	aiErr := r.reconcileAI(ctx, snapshot)

	return ctrl.Result{}, errors.Join(httpErr, aiErr)
}

func (r *Reconciler) reconcileHTTP(ctx context.Context, snapshot resources) error {
	if err := r.client.List(ctx, &snapshot.httpRoutes); err != nil {
		return fmt.Errorf("list HTTPRoutes: %w", err)
	}

	result := resolve(snapshot.gateways.Items, snapshot.httpRoutes.Items, snapshot.backends.Items)
	config := compile(result)

	if err := r.publisher.Publish(ctx, config); err != nil {
		return fmt.Errorf("publish HTTP data plane configuration: %w", err)
	}

	if err := r.updateGatewayStatuses(ctx, snapshot.gateways.Items, result.gatewayConflicts); err != nil {
		return err
	}

	if err := r.updateHTTPRouteStatuses(ctx, snapshot.httpRoutes.Items, result.parents); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) reconcileAI(ctx context.Context, snapshot resources) error {
	snapshot, err := r.readAIResources(ctx, snapshot)
	if err != nil {
		return err
	}

	config, err := compileAI(snapshot)
	if err != nil {
		return fmt.Errorf("compile AI data plane configuration: %w", err)
	}

	data, err := aidataplane.Encode(config)
	if err != nil {
		return fmt.Errorf("encode AI data plane configuration: %w", err)
	}

	// 两个输出独立尝试；不把一次 Cache 更新当作 Envoy 已接收，不增加全局 ACK 协调。
	xdsErr := r.publisher.PublishAI(ctx, config)
	if xdsErr != nil {
		xdsErr = fmt.Errorf("publish AI xDS configuration: %w", xdsErr)
	}

	configErr := r.aiConfig.write(ctx, data)
	if configErr != nil {
		configErr = fmt.Errorf("publish AI data plane configuration: %w", configErr)
	}

	return errors.Join(xdsErr, configErr)
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

func acceptedCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type:               gatewayv1alpha1.ConditionAccepted,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: generation,
		Reason:             "Accepted",
		Message:            "accepted into the shared data plane configuration",
	}
}
