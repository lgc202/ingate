// Package gateway 按资源作用域解析引用，编译共享 HTTP 与 AI 数据面配置。
package gateway

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
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

// Publisher 分别替换 HTTP 与 AI 期望配置，在适配器内维护两类发现缓存。
// 构建失败保留该路最近有效的结果；通知失败可能已更新缓存，必须重试而非假定回滚。
// 成功不代表 Envoy 或 AI Proxy 已接收或 ACK。
type Publisher interface {
	Publish(context.Context, dataplane.Config) error
	PublishAI(context.Context, aidataplane.Config, []*aidataplanev1.Resource) error
}

// Reconciler 协调六种领域资源，分别发布共享 HTTP 与 AI 配置。
type Reconciler struct {
	client    client.Client
	publisher Publisher
	sources   *compilationSource
	// aiResources 与单 Worker 一起串行使用，发布器只读取已完成的消息。
	aiResources aiResourceTranslation
}

// New 创建 Reconciler；Controller 的运行生命周期由 Manager 管理。
// SetupWithManager 注册计算集合，框架在初始同步完成后才调用 Reconcile。
func New(
	apiClient client.Client,
	publisher Publisher,
) *Reconciler {
	return &Reconciler{
		client:    apiClient,
		publisher: publisher,
	}
}

// SetupWithManager 通过 krt 运行投影合并六类领域资源事件。
// 所有源完成初始同步后才启动 Worker；空资源集合也主动协调一次。
func (r *Reconciler) SetupWithManager(manager ctrl.Manager) error {
	r.sources = newCompilationSource(manager.GetCache())

	return ctrl.NewControllerManagedBy(manager).
		Named("gateway").
		WatchesRawSource(r.sources).
		WatchesRawSource(source.Func(enqueueInitialReconcile)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 从已同步的派生集合组装两份完整配置；请求键只负责合并事件。
// 两路输出不是事务，一路失败不阻断另一路；错误交给工作队列限速重试。
func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	snapshot := r.sources.readPublication()

	httpErr := r.reconcileHTTP(ctx, snapshot)
	aiErr := r.reconcileAI(ctx, snapshot)

	return ctrl.Result{}, errors.Join(httpErr, aiErr)
}

func (r *Reconciler) reconcileHTTP(ctx context.Context, snapshot publication) error {
	config, err := snapshot.httpConfig()
	if err != nil {
		return fmt.Errorf("assemble HTTP data plane configuration: %w", err)
	}

	if err := r.publisher.Publish(ctx, config); err != nil {
		return fmt.Errorf("publish HTTP data plane configuration: %w", err)
	}

	if err := r.updateGatewayStatuses(ctx, snapshot.selection.gateways, snapshot.selection.conflicts); err != nil {
		return err
	}

	var routes []*httpRoute
	var parents []httpRouteParentResolution
	for _, status := range snapshot.statuses {
		routes = append(routes, status.route)
		parents = append(parents, status.parents...)
	}

	if err := r.updateHTTPRouteStatuses(ctx, routes, parents); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) reconcileAI(ctx context.Context, snapshot publication) error {
	config, err := snapshot.aiConfig()
	if err != nil {
		return fmt.Errorf("compile AI data plane configuration: %w", err)
	}

	translation, err := r.aiResources.compile(snapshot, config)
	if err != nil {
		return fmt.Errorf("compile AI Proxy data plane resources: %w", err)
	}

	// 只登记构建有效的候选；发布失败仍重试最新期望，不回滚可能已安装的缓存。
	r.aiResources = translation
	if err := r.publisher.PublishAI(ctx, config, translation.resources); err != nil {
		return fmt.Errorf("publish AI xDS configuration: %w", err)
	}

	return nil
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
