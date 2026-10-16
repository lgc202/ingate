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

const mcpRouteInitialSyncName = "_mcproute-initial-sync"

// MCPRouteReconciler 将 MCPRoute 同步到 MySQL 查询模型。
type MCPRouteReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewMCPRoute 创建 MCPRoute 查询模型 Reconciler。
func NewMCPRoute(apiClient client.Client, store *mysql.Store) *MCPRouteReconciler {
	return &MCPRouteReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 MCPRoute 协调和启动全量对账。
func (r *MCPRouteReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("mcproute-projection").
		For(&gatewayv1alpha1.MCPRoute{}).
		WatchesRawSource(source.Func(enqueueMCPRouteInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按命名空间和名称同步一个 MCPRoute，或执行启动全量对账。
func (r *MCPRouteReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Namespace == "" && request.Name == mcpRouteInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}

	return ctrl.Result{}, r.syncMCPRoute(ctx, request.NamespacedName)
}

func (r *MCPRouteReconciler) syncMCPRoute(ctx context.Context, key types.NamespacedName) error {
	var route gatewayv1alpha1.MCPRoute
	if err := r.client.Get(ctx, key, &route); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get MCPRoute %q: %w", key.String(), err)
		}

		if err := r.store.DeleteMCPRoute(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete MCPRoute %q from MySQL: %w", key.String(), err)
		}

		return nil
	}

	if err := r.store.UpsertMCPRoute(ctx, mcpRouteToMySQL(&route)); err != nil {
		return fmt.Errorf("sync MCPRoute %q to MySQL: %w", key.String(), err)
	}

	return nil
}

func (r *MCPRouteReconciler) syncAll(ctx context.Context) error {
	var routes gatewayv1alpha1.MCPRouteList
	if err := r.client.List(ctx, &routes); err != nil {
		return fmt.Errorf("list MCPRoutes for initial sync: %w", err)
	}

	current := make(map[types.NamespacedName]struct{}, len(routes.Items))
	for i := range routes.Items {
		route := &routes.Items[i]
		if err := r.store.UpsertMCPRoute(ctx, mcpRouteToMySQL(route)); err != nil {
			return fmt.Errorf("sync MCPRoute %q to MySQL: %w", client.ObjectKeyFromObject(route).String(), err)
		}

		current[client.ObjectKeyFromObject(route)] = struct{}{}
	}

	storedKeys, err := r.store.ListMCPRouteKeys(ctx)
	if err != nil {
		return err
	}

	for _, key := range storedKeys {
		if _, exists := current[key]; exists {
			continue
		}

		if err := r.store.DeleteMCPRoute(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete stale MCPRoute %q from MySQL: %w", key.String(), err)
		}
	}

	return nil
}

func mcpRouteToMySQL(route *gatewayv1alpha1.MCPRoute) mysql.MCPRoute {
	// API Server 已保证单个后端并默认两端路径；投影只转换保存结果。
	ref := route.Spec.BackendRefs[0]

	result := mysql.MCPRoute{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: route.Spec.DisplayName,
		Path:        *route.Spec.Path,
		BackendName: ref.Name,
		BackendPath: *ref.Path,
		Version:     route.ResourceVersion,
		CreatedAt:   route.CreationTimestamp.UTC(),
	}
	for _, parent := range route.Spec.ParentRefs {
		result.Parents = append(result.Parents, mysql.MCPRouteParent{
			Name:        parent.Name,
			SectionName: parent.SectionName,
		})
	}

	return result
}

func enqueueMCPRouteInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: mcpRouteInitialSyncName}})
	return nil
}
