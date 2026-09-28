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

const routeInitialSyncName = "_route-initial-sync"

// RouteReconciler 将 Route 同步到 MySQL 查询模型。
type RouteReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewRoute 创建 Route 查询模型 Reconciler。
func NewRoute(apiClient client.Client, store *mysql.Store) *RouteReconciler {
	return &RouteReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 Route 协调和启动全量对账。
func (r *RouteReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("route-projection").
		For(&gatewayv1alpha1.Route{}).
		WatchesRawSource(source.Func(enqueueRouteInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按名称同步一个 Route，或执行启动全量对账。
func (r *RouteReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Name == routeInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}
	return ctrl.Result{}, r.syncRoute(ctx, request.Name)
}

func (r *RouteReconciler) syncRoute(ctx context.Context, name string) error {
	var route gatewayv1alpha1.Route
	if err := r.client.Get(ctx, types.NamespacedName{Name: name}, &route); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get Route %q: %w", name, err)
		}
		if err := r.store.DeleteRoute(ctx, name); err != nil {
			return fmt.Errorf("delete Route %q from MySQL: %w", name, err)
		}
		return nil
	}

	if err := r.store.UpsertRoute(ctx, routeToMySQL(&route)); err != nil {
		return fmt.Errorf("sync Route %q to MySQL: %w", name, err)
	}

	return nil
}

func (r *RouteReconciler) syncAll(ctx context.Context) error {
	var routes gatewayv1alpha1.RouteList
	if err := r.client.List(ctx, &routes); err != nil {
		return fmt.Errorf("list Routes for initial sync: %w", err)
	}

	current := make(map[string]struct{}, len(routes.Items))
	for i := range routes.Items {
		route := &routes.Items[i]
		if err := r.store.UpsertRoute(ctx, routeToMySQL(route)); err != nil {
			return fmt.Errorf("sync Route %q to MySQL: %w", route.Name, err)
		}
		current[route.Name] = struct{}{}
	}

	storedNames, err := r.store.ListRouteNames(ctx)
	if err != nil {
		return err
	}

	for _, name := range storedNames {
		if _, exists := current[name]; exists {
			continue
		}
		if err := r.store.DeleteRoute(ctx, name); err != nil {
			return fmt.Errorf("delete stale Route %q from MySQL: %w", name, err)
		}
	}

	return nil
}

func routeToMySQL(route *gatewayv1alpha1.Route) mysql.Route {
	return mysql.Route{
		Name:            route.Name,
		GatewayName:     route.Spec.GatewayName,
		ListenerName:    route.Spec.ListenerName,
		PathType:        string(route.Spec.Path.Type),
		PathValue:       route.Spec.Path.Value,
		UpstreamName:    route.Spec.UpstreamName,
		ResourceVersion: route.ResourceVersion,
		CreatedAt:       route.CreationTimestamp.UTC(),
	}
}

func enqueueRouteInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: routeInitialSyncName}})
	return nil
}
