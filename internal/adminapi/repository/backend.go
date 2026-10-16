package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
)

// Backend 将写入交给 API Server，将查询交给 MySQL 投影。
type Backend struct {
	client gatewayclient.BackendsGetter
	store  *mysql.Store
}

// BackendInfo 保存管理查询及创建、更新响应所需的后端信息，不表示完整的 Kubernetes 资源。
type BackendInfo struct {
	Namespace   string
	Name        string
	DisplayName string
	Endpoints   []gatewayv1alpha1.Endpoint
	CreateTime  time.Time
	Version     string
}

// BackendPage 保存一页产品查询视图及应用相同筛选条件的总数。
type BackendPage struct {
	Items      []BackendInfo
	TotalCount int64
}

// NewBackend 创建 Backend Repository。
func NewBackend(client gatewayclient.BackendsGetter, store *mysql.Store) *Backend {
	return &Backend{client: client, store: store}
}

// Create 创建 Backend，并返回 API Server 保存后的业务状态。
func (r *Backend) Create(
	ctx context.Context,
	backend *gatewayv1alpha1.Backend,
) (BackendInfo, error) {
	created, err := r.client.Backends(backend.Namespace).Create(ctx, backend.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		return BackendInfo{}, translateResourceError(err)
	}

	return backendFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 Backend。
func (r *Backend) Get(
	ctx context.Context,
	namespace, name string,
) (BackendInfo, error) {
	backend, err := r.store.GetBackend(ctx, namespace, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return BackendInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}

		return BackendInfo{}, err
	}

	return backendFromMySQL(backend), nil
}

// List 从 MySQL 查询模型按页码读取 Backend。
func (r *Backend) List(
	ctx context.Context,
	params ResourceListParams,
) (BackendPage, error) {
	result, err := r.store.ListBackends(ctx, mysql.ResourceListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		return BackendPage{}, err
	}

	backends := make([]BackendInfo, len(result.Items))
	for i, backend := range result.Items {
		backends[i] = backendFromMySQL(backend)
	}

	return BackendPage{Items: backends, TotalCount: result.TotalCount}, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 Backend 的可编辑配置。
func (r *Backend) Update(
	ctx context.Context,
	backend *gatewayv1alpha1.Backend,
) (BackendInfo, error) {
	current, err := r.client.Backends(backend.Namespace).Get(ctx, backend.Name, metav1.GetOptions{})
	if err != nil {
		return BackendInfo{}, translateResourceError(err)
	}

	if current.ResourceVersion != backend.ResourceVersion {
		return BackendInfo{}, ErrConflict
	}

	current.Spec = backend.DeepCopy().Spec
	updated, err := r.client.Backends(backend.Namespace).Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return BackendInfo{}, translateResourceError(err)
	}

	return backendFromAPI(updated), nil
}

// Delete 删除指定 Backend。
func (r *Backend) Delete(ctx context.Context, namespace, name string) error {
	return translateResourceError(r.client.Backends(namespace).Delete(ctx, name, metav1.DeleteOptions{}))
}

func backendFromAPI(backend *gatewayv1alpha1.Backend) BackendInfo {
	spec := backend.DeepCopy().Spec

	return BackendInfo{
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: spec.DisplayName,
		Endpoints:   spec.Endpoints,
		CreateTime:  backend.CreationTimestamp.Time,
		Version:     backend.ResourceVersion,
	}
}

func backendFromMySQL(backend mysql.Backend) BackendInfo {
	endpoints := make([]gatewayv1alpha1.Endpoint, len(backend.Endpoints))
	for i, endpoint := range backend.Endpoints {
		endpoints[i] = gatewayv1alpha1.Endpoint{
			Address: endpoint.Address,
			Port:    int32(endpoint.Port),
			Weight:  endpoint.Weight,
		}
	}

	return BackendInfo{
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: backend.DisplayName,
		Endpoints:   endpoints,
		CreateTime:  backend.CreatedAt,
		Version:     backend.Version,
	}
}
