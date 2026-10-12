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

const aiBackendInitialSyncName = "_aibackend-initial-sync"

// AIBackendReconciler 将 AIBackend 同步到 MySQL 查询模型。
type AIBackendReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewAIBackend 创建 AIBackend 查询模型 Reconciler。
func NewAIBackend(apiClient client.Client, store *mysql.Store) *AIBackendReconciler {
	return &AIBackendReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 AIBackend 协调和启动全量对账。
func (r *AIBackendReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("aibackend-projection").
		For(&gatewayv1alpha1.AIBackend{}).
		WatchesRawSource(source.Func(enqueueAIBackendInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按名称同步一个 AIBackend，或执行启动全量对账。
func (r *AIBackendReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Name == aiBackendInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}

	return ctrl.Result{}, r.syncAIBackend(ctx, request.Name)
}

func (r *AIBackendReconciler) syncAIBackend(ctx context.Context, name string) error {
	var backend gatewayv1alpha1.AIBackend
	if err := r.client.Get(ctx, types.NamespacedName{Name: name}, &backend); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get AIBackend %q: %w", name, err)
		}
		if err := r.store.DeleteAIBackend(ctx, name); err != nil {
			return fmt.Errorf("delete AIBackend %q from MySQL: %w", name, err)
		}
		return nil
	}

	if err := r.store.UpsertAIBackend(ctx, aiBackendToMySQL(&backend)); err != nil {
		return fmt.Errorf("sync AIBackend %q to MySQL: %w", name, err)
	}

	return nil
}

func (r *AIBackendReconciler) syncAll(ctx context.Context) error {
	var backends gatewayv1alpha1.AIBackendList
	if err := r.client.List(ctx, &backends); err != nil {
		return fmt.Errorf("list AIBackends for initial sync: %w", err)
	}

	current := make(map[string]struct{}, len(backends.Items))
	for i := range backends.Items {
		backend := &backends.Items[i]
		if err := r.store.UpsertAIBackend(ctx, aiBackendToMySQL(backend)); err != nil {
			return fmt.Errorf("sync AIBackend %q to MySQL: %w", backend.Name, err)
		}
		current[backend.Name] = struct{}{}
	}

	storedNames, err := r.store.ListAIBackendNames(ctx)
	if err != nil {
		return err
	}

	for _, name := range storedNames {
		if _, exists := current[name]; exists {
			continue
		}
		if err := r.store.DeleteAIBackend(ctx, name); err != nil {
			return fmt.Errorf("delete stale AIBackend %q from MySQL: %w", name, err)
		}
	}

	return nil
}

func aiBackendToMySQL(backend *gatewayv1alpha1.AIBackend) mysql.AIBackend {
	return mysql.AIBackend{
		Name:         backend.Name,
		DisplayName:  backend.Spec.DisplayName,
		BackendName:  backend.Spec.BackendRef.Name,
		SchemaName:   string(backend.Spec.Schema.Name),
		SchemaPrefix: backend.Spec.Schema.Prefix,
		Version:      backend.ResourceVersion,
		CreatedAt:    backend.CreationTimestamp.UTC(),
	}
}

func enqueueAIBackendInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: aiBackendInitialSyncName}})
	return nil
}
