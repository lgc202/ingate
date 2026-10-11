// Package projection 将声明式资源同步到面向产品查询的 MySQL 模型。
package projection

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
)

const gatewayInitialSyncName = "_gateway-initial-sync"

// GatewayReconciler 将 Gateway 同步到 MySQL 查询模型。
type GatewayReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewGateway 创建 Gateway 查询模型 Reconciler。
func NewGateway(apiClient client.Client, store *mysql.Store) *GatewayReconciler {
	return &GatewayReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 Gateway 协调和启动全量对账。
func (r *GatewayReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("gateway-projection").
		For(&gatewayv1alpha1.Gateway{}).
		WatchesRawSource(source.Func(enqueueGatewayInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按名称同步一个 Gateway，或执行启动全量对账。
func (r *GatewayReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Name == gatewayInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}
	return ctrl.Result{}, r.syncGateway(ctx, request.Name)
}

func (r *GatewayReconciler) syncGateway(ctx context.Context, name string) error {
	var gateway gatewayv1alpha1.Gateway
	if err := r.client.Get(ctx, types.NamespacedName{Name: name}, &gateway); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get Gateway %q: %w", name, err)
		}
		if err := r.store.DeleteGateway(ctx, name); err != nil {
			return fmt.Errorf("delete Gateway %q from MySQL: %w", name, err)
		}
		return nil
	}

	if err := r.store.UpsertGateway(ctx, gatewayToMySQL(&gateway)); err != nil {
		return fmt.Errorf("sync Gateway %q to MySQL: %w", name, err)
	}

	return nil
}

func (r *GatewayReconciler) syncAll(ctx context.Context) error {
	var gateways gatewayv1alpha1.GatewayList
	if err := r.client.List(ctx, &gateways); err != nil {
		return fmt.Errorf("list Gateways for initial sync: %w", err)
	}

	current := make(map[string]struct{}, len(gateways.Items))
	for i := range gateways.Items {
		gateway := &gateways.Items[i]
		if err := r.store.UpsertGateway(ctx, gatewayToMySQL(gateway)); err != nil {
			return fmt.Errorf("sync Gateway %q to MySQL: %w", gateway.Name, err)
		}
		current[gateway.Name] = struct{}{}
	}

	storedNames, err := r.store.ListGatewayNames(ctx)
	if err != nil {
		return err
	}

	for _, name := range storedNames {
		if _, exists := current[name]; exists {
			continue
		}
		if err := r.store.DeleteGateway(ctx, name); err != nil {
			return fmt.Errorf("delete stale Gateway %q from MySQL: %w", name, err)
		}
	}

	return nil
}

func gatewayToMySQL(gateway *gatewayv1alpha1.Gateway) mysql.Gateway {
	listeners := make([]mysql.GatewayListener, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		listeners[i] = mysql.GatewayListener{
			Name:     listener.Name,
			Protocol: string(listener.Protocol),
			Port:     uint16(listener.Port),
			Hostname: listener.Hostname,
		}
	}

	return mysql.Gateway{
		Name:        gateway.Name,
		DisplayName: gateway.Spec.DisplayName,
		Version:     gateway.ResourceVersion,
		CreatedAt:   gateway.CreationTimestamp.UTC(),
		Listeners:   listeners,
	}
}

func enqueueGatewayInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: gatewayInitialSyncName}})
	return nil
}
