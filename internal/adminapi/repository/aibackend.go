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

// AIBackend 将写入交给 API Server，将查询交给 MySQL 投影。
type AIBackend struct {
	client gatewayclient.AIBackendsGetter
	store  *mysql.Store
}

// AIBackendInfo 保存管理查询及创建、更新响应所需的模型服务信息，不表示完整的 Kubernetes 资源。
type AIBackendInfo struct {
	Namespace   string
	Name        string
	DisplayName string
	BackendRef  gatewayv1alpha1.BackendReference
	Schema      gatewayv1alpha1.APISchema
	CreateTime  time.Time
	Version     string
}

// AIBackendPage 保存一页产品查询视图及应用相同筛选条件的总数。
type AIBackendPage struct {
	Items      []AIBackendInfo
	TotalCount int64
}

// NewAIBackend 创建 AIBackend Repository。
func NewAIBackend(client gatewayclient.AIBackendsGetter, store *mysql.Store) *AIBackend {
	return &AIBackend{client: client, store: store}
}

// Create 创建 AIBackend，并返回 API Server 保存后的业务状态。
func (r *AIBackend) Create(
	ctx context.Context,
	backend *gatewayv1alpha1.AIBackend,
) (AIBackendInfo, error) {
	created, err := r.client.AIBackends(backend.Namespace).Create(ctx, backend.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		return AIBackendInfo{}, translateResourceError(err)
	}

	return aiBackendFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 AIBackend。
func (r *AIBackend) Get(
	ctx context.Context,
	namespace, name string,
) (AIBackendInfo, error) {
	backend, err := r.store.GetAIBackend(ctx, namespace, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return AIBackendInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return AIBackendInfo{}, err
	}

	return aiBackendFromMySQL(backend), nil
}

// List 从 MySQL 查询模型按页码读取 AIBackend。
func (r *AIBackend) List(
	ctx context.Context,
	params ResourceListParams,
) (AIBackendPage, error) {
	result, err := r.store.ListAIBackends(ctx, mysql.ResourceListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		return AIBackendPage{}, err
	}

	backends := make([]AIBackendInfo, len(result.Items))
	for i, backend := range result.Items {
		backends[i] = aiBackendFromMySQL(backend)
	}

	return AIBackendPage{Items: backends, TotalCount: result.TotalCount}, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 AIBackend 的可编辑配置。
func (r *AIBackend) Update(
	ctx context.Context,
	backend *gatewayv1alpha1.AIBackend,
) (AIBackendInfo, error) {
	current, err := r.client.AIBackends(backend.Namespace).Get(ctx, backend.Name, metav1.GetOptions{})
	if err != nil {
		return AIBackendInfo{}, translateResourceError(err)
	}

	if current.ResourceVersion != backend.ResourceVersion {
		return AIBackendInfo{}, ErrConflict
	}

	current.Spec = backend.DeepCopy().Spec
	updated, err := r.client.AIBackends(backend.Namespace).Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return AIBackendInfo{}, translateResourceError(err)
	}

	return aiBackendFromAPI(updated), nil
}

// Delete 删除指定 AIBackend。
func (r *AIBackend) Delete(ctx context.Context, namespace, name string) error {
	return translateResourceError(r.client.AIBackends(namespace).Delete(ctx, name, metav1.DeleteOptions{}))
}

func aiBackendFromAPI(backend *gatewayv1alpha1.AIBackend) AIBackendInfo {
	spec := backend.Spec
	return AIBackendInfo{
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: spec.DisplayName,
		BackendRef:  spec.BackendRef,
		Schema:      spec.Schema,
		CreateTime:  backend.CreationTimestamp.Time,
		Version:     backend.ResourceVersion,
	}
}

func aiBackendFromMySQL(backend mysql.AIBackend) AIBackendInfo {
	return AIBackendInfo{
		Namespace:   backend.Namespace,
		Name:        backend.Name,
		DisplayName: backend.DisplayName,
		BackendRef:  gatewayv1alpha1.BackendReference{Name: backend.BackendName},
		Schema: gatewayv1alpha1.APISchema{
			Name:   gatewayv1alpha1.APISchemaName(backend.SchemaName),
			Prefix: backend.SchemaPrefix,
		},
		CreateTime: backend.CreatedAt,
		Version:    backend.Version,
	}
}
