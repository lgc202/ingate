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

const modelRouteInitialSyncName = "_modelroute-initial-sync"

// ModelRouteReconciler 将 ModelRoute 同步到 MySQL 查询模型。
type ModelRouteReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewModelRoute 创建 ModelRoute 查询模型 Reconciler。
func NewModelRoute(apiClient client.Client, store *mysql.Store) *ModelRouteReconciler {
	return &ModelRouteReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 ModelRoute 协调和启动全量对账。
func (r *ModelRouteReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("modelroute-projection").
		For(&gatewayv1alpha1.ModelRoute{}).
		WatchesRawSource(source.Func(enqueueModelRouteInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按命名空间和名称同步一个 ModelRoute，或执行启动全量对账。
func (r *ModelRouteReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Namespace == "" && request.Name == modelRouteInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}
	return ctrl.Result{}, r.syncModelRoute(ctx, request.NamespacedName)
}

func (r *ModelRouteReconciler) syncModelRoute(ctx context.Context, key types.NamespacedName) error {
	var route gatewayv1alpha1.ModelRoute
	if err := r.client.Get(ctx, key, &route); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get ModelRoute %q: %w", key.String(), err)
		}
		if err := r.store.DeleteModelRoute(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete ModelRoute %q from MySQL: %w", key.String(), err)
		}
		return nil
	}

	if err := r.store.UpsertModelRoute(ctx, modelRouteToMySQL(&route)); err != nil {
		return fmt.Errorf("sync ModelRoute %q to MySQL: %w", key.String(), err)
	}

	return nil
}

func (r *ModelRouteReconciler) syncAll(ctx context.Context) error {
	var routes gatewayv1alpha1.ModelRouteList
	if err := r.client.List(ctx, &routes); err != nil {
		return fmt.Errorf("list ModelRoutes for initial sync: %w", err)
	}

	current := make(map[types.NamespacedName]struct{}, len(routes.Items))
	for i := range routes.Items {
		route := &routes.Items[i]
		if err := r.store.UpsertModelRoute(ctx, modelRouteToMySQL(route)); err != nil {
			return fmt.Errorf("sync ModelRoute %q to MySQL: %w", client.ObjectKeyFromObject(route).String(), err)
		}
		current[client.ObjectKeyFromObject(route)] = struct{}{}
	}

	storedKeys, err := r.store.ListModelRouteKeys(ctx)
	if err != nil {
		return err
	}

	for _, key := range storedKeys {
		if _, exists := current[key]; exists {
			continue
		}
		if err := r.store.DeleteModelRoute(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete stale ModelRoute %q from MySQL: %w", key.String(), err)
		}
	}

	return nil
}

func modelRouteToMySQL(route *gatewayv1alpha1.ModelRoute) mysql.ModelRoute {
	result := mysql.ModelRoute{
		Namespace:   route.Namespace,
		Name:        route.Name,
		DisplayName: route.Spec.DisplayName,
		Version:     route.ResourceVersion,
		CreatedAt:   route.CreationTimestamp.UTC(),
	}

	for _, ref := range route.Spec.ParentRefs {
		result.Parents = append(result.Parents, mysql.ModelRouteParent{Name: ref.Name, SectionName: ref.SectionName})
	}
	for _, rule := range route.Spec.Rules {
		target := mysql.ModelRouteRule{Name: rule.Name}
		for _, match := range rule.Matches {
			condition := mysql.ModelRouteMatch{}
			for _, header := range match.Headers {
				condition.Headers = append(condition.Headers, mysql.ModelRouteHeaderMatch{
					Name:  header.Name,
					Type:  string(header.Type),
					Value: header.Value,
				})
			}
			target.Matches = append(target.Matches, condition)
		}
		for _, ref := range rule.BackendRefs {
			target.BackendRefs = append(target.BackendRefs, mysql.ModelRouteBackendRef{
				Name:              ref.Name,
				ModelNameOverride: ref.ModelNameOverride,
			})
		}
		result.Rules = append(result.Rules, target)
	}

	return result
}

func enqueueModelRouteInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: modelRouteInitialSyncName}})
	return nil
}
