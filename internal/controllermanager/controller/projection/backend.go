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

const backendInitialSyncName = "_backend-initial-sync"

// BackendReconciler 将 Backend 同步到 MySQL 查询模型。
type BackendReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewBackend 创建 Backend 查询模型 Reconciler。
func NewBackend(apiClient client.Client, store *mysql.Store) *BackendReconciler {
	return &BackendReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 Backend 协调和启动全量对账。
func (r *BackendReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("backend-projection").
		For(&gatewayv1alpha1.Backend{}).
		WatchesRawSource(source.Func(enqueueBackendInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按命名空间和名称同步一个 Backend，或执行启动全量对账。
func (r *BackendReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Namespace == "" && request.Name == backendInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}

	return ctrl.Result{}, r.syncBackend(ctx, request.NamespacedName)
}

func (r *BackendReconciler) syncBackend(ctx context.Context, key types.NamespacedName) error {
	var backend gatewayv1alpha1.Backend
	if err := r.client.Get(ctx, key, &backend); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get Backend %q: %w", key.String(), err)
		}

		if err := r.store.DeleteBackend(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete Backend %q from MySQL: %w", key.String(), err)
		}

		return nil
	}

	if err := r.store.UpsertBackend(ctx, backendToMySQL(&backend)); err != nil {
		return fmt.Errorf("sync Backend %q to MySQL: %w", key.String(), err)
	}

	return nil
}

func (r *BackendReconciler) syncAll(ctx context.Context) error {
	var backends gatewayv1alpha1.BackendList
	if err := r.client.List(ctx, &backends); err != nil {
		return fmt.Errorf("list Backends for initial sync: %w", err)
	}

	current := make(map[types.NamespacedName]struct{}, len(backends.Items))
	for i := range backends.Items {
		backend := &backends.Items[i]
		if err := r.store.UpsertBackend(ctx, backendToMySQL(backend)); err != nil {
			return fmt.Errorf("sync Backend %q to MySQL: %w", client.ObjectKeyFromObject(backend).String(), err)
		}

		current[client.ObjectKeyFromObject(backend)] = struct{}{}
	}

	storedKeys, err := r.store.ListBackendKeys(ctx)
	if err != nil {
		return err
	}

	for _, key := range storedKeys {
		if _, exists := current[key]; exists {
			continue
		}

		if err := r.store.DeleteBackend(ctx, key.Namespace, key.Name); err != nil {
			return fmt.Errorf("delete stale Backend %q from MySQL: %w", key.String(), err)
		}
	}

	return nil
}

func backendToMySQL(backend *gatewayv1alpha1.Backend) mysql.Backend {
	endpoints := make([]mysql.BackendEndpoint, len(backend.Spec.Endpoints))
	for i, endpoint := range backend.Spec.Endpoints {
		endpoints[i] = mysql.BackendEndpoint{
			Address: endpoint.Address,
			Port:    uint16(endpoint.Port),
			Weight:  endpoint.Weight,
		}
	}

	return mysql.Backend{
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: backend.Spec.DisplayName,
		Version:     backend.ResourceVersion,
		CreatedAt:   backend.CreationTimestamp.UTC(),
		Endpoints:   endpoints,
	}
}

func enqueueBackendInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: backendInitialSyncName}})
	return nil
}
