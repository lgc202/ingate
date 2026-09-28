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

const upstreamInitialSyncName = "_upstream-initial-sync"

// UpstreamReconciler 将 Upstream 同步到 MySQL 查询模型。
type UpstreamReconciler struct {
	client client.Client
	store  *mysql.Store
}

// NewUpstream 创建 Upstream 查询模型 Reconciler。
func NewUpstream(apiClient client.Client, store *mysql.Store) *UpstreamReconciler {
	return &UpstreamReconciler{client: apiClient, store: store}
}

// SetupWithManager 注册逐 Upstream 协调和启动全量对账。
func (r *UpstreamReconciler) SetupWithManager(manager ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(manager).
		Named("upstream-projection").
		For(&gatewayv1alpha1.Upstream{}).
		WatchesRawSource(source.Func(enqueueUpstreamInitialSync)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}

// Reconcile 按名称同步一个 Upstream，或执行启动全量对账。
func (r *UpstreamReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	if request.Name == upstreamInitialSyncName {
		return ctrl.Result{}, r.syncAll(ctx)
	}
	return ctrl.Result{}, r.syncUpstream(ctx, request.Name)
}

func (r *UpstreamReconciler) syncUpstream(ctx context.Context, name string) error {
	var upstream gatewayv1alpha1.Upstream
	if err := r.client.Get(ctx, types.NamespacedName{Name: name}, &upstream); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get Upstream %q: %w", name, err)
		}
		if err := r.store.DeleteUpstream(ctx, name); err != nil {
			return fmt.Errorf("delete Upstream %q from MySQL: %w", name, err)
		}
		return nil
	}

	if err := r.store.UpsertUpstream(ctx, upstreamToMySQL(&upstream)); err != nil {
		return fmt.Errorf("sync Upstream %q to MySQL: %w", name, err)
	}

	return nil
}

func (r *UpstreamReconciler) syncAll(ctx context.Context) error {
	var upstreams gatewayv1alpha1.UpstreamList
	if err := r.client.List(ctx, &upstreams); err != nil {
		return fmt.Errorf("list Upstreams for initial sync: %w", err)
	}

	current := make(map[string]struct{}, len(upstreams.Items))
	for i := range upstreams.Items {
		upstream := &upstreams.Items[i]
		if err := r.store.UpsertUpstream(ctx, upstreamToMySQL(upstream)); err != nil {
			return fmt.Errorf("sync Upstream %q to MySQL: %w", upstream.Name, err)
		}
		current[upstream.Name] = struct{}{}
	}

	storedNames, err := r.store.ListUpstreamNames(ctx)
	if err != nil {
		return err
	}

	for _, name := range storedNames {
		if _, exists := current[name]; exists {
			continue
		}
		if err := r.store.DeleteUpstream(ctx, name); err != nil {
			return fmt.Errorf("delete stale Upstream %q from MySQL: %w", name, err)
		}
	}

	return nil
}

func upstreamToMySQL(upstream *gatewayv1alpha1.Upstream) mysql.Upstream {
	endpoints := make([]mysql.UpstreamEndpoint, len(upstream.Spec.Endpoints))
	for i, endpoint := range upstream.Spec.Endpoints {
		endpoints[i] = mysql.UpstreamEndpoint{
			Address: endpoint.Address,
			Port:    uint16(endpoint.Port),
			Weight:  endpoint.Weight,
		}
	}

	return mysql.Upstream{
		Name:            upstream.Name,
		ResourceVersion: upstream.ResourceVersion,
		CreatedAt:       upstream.CreationTimestamp.UTC(),
		Endpoints:       endpoints,
	}
}

func enqueueUpstreamInitialSync(
	_ context.Context,
	queue workqueue.TypedRateLimitingInterface[reconcile.Request],
) error {
	queue.Add(reconcile.Request{NamespacedName: types.NamespacedName{Name: upstreamInitialSyncName}})
	return nil
}
