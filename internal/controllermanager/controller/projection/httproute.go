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

const routeInitialSyncName = "_httproute-initial-sync"

// HTTPRouteReconciler 将 HTTPRoute 同步到 MySQL 查询模型。
type HTTPRouteReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewHTTPRoute 创建 HTTPRoute 查询模型 Reconciler。
func NewHTTPRoute(apiClient client.Client, store *mysql.Store) *HTTPRouteReconciler {
	return &HTTPRouteReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 HTTPRoute 协调和启动全量对账。
func (r *HTTPRouteReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("httproute-projection").
		For(&gatewayv1alpha1.HTTPRoute{}).
		WatchesRawSource(source.Func(enqueueHTTPRouteInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按命名空间和名称同步一个 HTTPRoute，或执行启动全量对账。
func (r *HTTPRouteReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Namespace == "" && request.Name == routeInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}

	return ctrl.Result{}, r.syncHTTPRoute(ctx, request.NamespacedName)
}

func (r *HTTPRouteReconciler) syncHTTPRoute(ctx context.Context, key types.NamespacedName) error {
	var route gatewayv1alpha1.HTTPRoute
	if err := r.client.Get(ctx, key, &route); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get HTTPRoute %q: %w", key.String(), err)
		}

		if err := r.store.DeleteHTTPRoute(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete HTTPRoute %q from MySQL: %w", key.String(), err)
		}

		return nil
	}

	if err := r.store.UpsertHTTPRoute(ctx, routeToMySQL(&route)); err != nil {
		return fmt.Errorf("sync HTTPRoute %q to MySQL: %w", key.String(), err)
	}

	return nil
}

func (r *HTTPRouteReconciler) syncAll(ctx context.Context) error {
	var routes gatewayv1alpha1.HTTPRouteList
	if err := r.client.List(ctx, &routes); err != nil {
		return fmt.Errorf("list HTTPRoutes for initial sync: %w", err)
	}

	current := make(map[types.NamespacedName]struct{}, len(routes.Items))
	for i := range routes.Items {
		route := &routes.Items[i]
		if err := r.store.UpsertHTTPRoute(ctx, routeToMySQL(route)); err != nil {
			return fmt.Errorf("sync HTTPRoute %q to MySQL: %w", client.ObjectKeyFromObject(route).String(), err)
		}

		current[client.ObjectKeyFromObject(route)] = struct{}{}
	}

	storedKeys, err := r.store.ListHTTPRouteKeys(ctx)
	if err != nil {
		return err
	}

	for _, key := range storedKeys {
		if _, exists := current[key]; exists {
			continue
		}

		if err := r.store.DeleteHTTPRoute(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete stale HTTPRoute %q from MySQL: %w", key.String(), err)
		}
	}

	return nil
}

func routeToMySQL(route *gatewayv1alpha1.HTTPRoute) mysql.HTTPRoute {
	defaulted := route.DeepCopy()
	gatewayv1alpha1.SetDefaultsHTTPRoute(defaulted)
	result := mysql.HTTPRoute{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: route.Spec.DisplayName,
		Version:     route.ResourceVersion,
		CreatedAt:   route.CreationTimestamp.UTC(),
	}

	for _, ref := range defaulted.Spec.ParentRefs {
		result.Parents = append(result.Parents, mysql.HTTPRouteParent{Name: ref.Name, SectionName: ref.SectionName})
	}

	for _, rule := range defaulted.Spec.Rules {
		target := mysql.HTTPRouteRule{Name: rule.Name}
		for _, match := range rule.Matches {
			target.Matches = append(target.Matches, mysql.HTTPRouteMatch{
				PathType:  string(match.Path.Type),
				PathValue: match.Path.Value,
			})
		}

		for _, ref := range rule.BackendRefs {
			target.BackendRefs = append(target.BackendRefs, mysql.HTTPRouteBackendRef{Name: ref.Name})
		}

		result.Rules = append(result.Rules, target)
	}

	return result
}

func enqueueHTTPRouteInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: routeInitialSyncName}})
	return nil
}
