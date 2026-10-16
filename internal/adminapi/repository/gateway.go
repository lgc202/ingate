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

// Gateway 将写入交给 API Server，将查询交给 MySQL 投影。
type Gateway struct {
	client gatewayclient.GatewaysGetter
	store  *mysql.Store
}

// ResourceListParams 保存网关或后端列表的分页与展示名称包含匹配条件。
type ResourceListParams struct {
	Namespace   string
	PageSize    int32
	PageNum     int32
	DisplayName string
}

// GatewayInfo 保存管理查询及创建、更新响应所需的网关信息，不表示完整的 Kubernetes 资源。
type GatewayInfo struct {
	Namespace   string
	Name        string
	DisplayName string
	Listeners   []gatewayv1alpha1.Listener
	CreateTime  time.Time
	Version     string
}

// GatewayPage 保存一页产品查询视图及应用相同筛选条件的总数。
type GatewayPage struct {
	Items      []GatewayInfo
	TotalCount int64
}

// NewGateway 创建 Gateway Repository。
func NewGateway(client gatewayclient.GatewaysGetter, store *mysql.Store) *Gateway {
	return &Gateway{client: client, store: store}
}

// Create 创建 Gateway，并返回 API Server 保存后的业务状态。
func (r *Gateway) Create(
	ctx context.Context,
	gateway *gatewayv1alpha1.Gateway,
) (GatewayInfo, error) {
	created, err := r.client.Gateways(gateway.Namespace).Create(ctx, gateway.DeepCopy(), metav1.CreateOptions{})
	if err != nil {
		return GatewayInfo{}, translateResourceError(err)
	}

	return gatewayFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 Gateway。
func (r *Gateway) Get(
	ctx context.Context,
	namespace, name string,
) (GatewayInfo, error) {
	gateway, err := r.store.GetGateway(ctx, namespace, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return GatewayInfo{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}

		return GatewayInfo{}, err
	}

	return gatewayFromMySQL(gateway), nil
}

// List 从 MySQL 查询模型按页码读取 Gateway。
func (r *Gateway) List(
	ctx context.Context,
	params ResourceListParams,
) (GatewayPage, error) {
	result, err := r.store.ListGateways(ctx, mysql.ResourceListParams{
		Namespace:   params.Namespace,
		PageSize:    params.PageSize,
		PageNum:     params.PageNum,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		return GatewayPage{}, err
	}

	gateways := make([]GatewayInfo, len(result.Items))
	for i, gateway := range result.Items {
		gateways[i] = gatewayFromMySQL(gateway)
	}

	return GatewayPage{Items: gateways, TotalCount: result.TotalCount}, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 Gateway 的可编辑配置。
func (r *Gateway) Update(
	ctx context.Context,
	gateway *gatewayv1alpha1.Gateway,
) (GatewayInfo, error) {
	current, err := r.client.Gateways(gateway.Namespace).Get(ctx, gateway.Name, metav1.GetOptions{})
	if err != nil {
		return GatewayInfo{}, translateResourceError(err)
	}

	if current.ResourceVersion != gateway.ResourceVersion {
		return GatewayInfo{}, ErrConflict
	}

	current.Spec = gateway.DeepCopy().Spec
	updated, err := r.client.Gateways(gateway.Namespace).Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return GatewayInfo{}, translateResourceError(err)
	}

	return gatewayFromAPI(updated), nil
}

// Delete 删除指定 Gateway。
func (r *Gateway) Delete(ctx context.Context, namespace, name string) error {
	return translateResourceError(r.client.Gateways(namespace).Delete(ctx, name, metav1.DeleteOptions{}))
}

func gatewayFromAPI(gateway *gatewayv1alpha1.Gateway) GatewayInfo {
	spec := gateway.DeepCopy().Spec

	return GatewayInfo{
		Namespace:   gateway.Namespace,
		Name:        gateway.Name,
		DisplayName: spec.DisplayName,
		Listeners:   spec.Listeners,
		CreateTime:  gateway.CreationTimestamp.Time,
		Version:     gateway.ResourceVersion,
	}
}

func gatewayFromMySQL(gateway mysql.Gateway) GatewayInfo {
	listeners := make([]gatewayv1alpha1.Listener, len(gateway.Listeners))
	for i, listener := range gateway.Listeners {
		listeners[i] = gatewayv1alpha1.Listener{
			Name:     listener.Name,
			Protocol: gatewayv1alpha1.Protocol(listener.Protocol),
			Port:     int32(listener.Port),
			Hostname: listener.Hostname,
		}
	}

	return GatewayInfo{
		Namespace:   gateway.Namespace,
		Name:        gateway.Name,
		DisplayName: gateway.DisplayName,
		Listeners:   listeners,
		CreateTime:  gateway.CreatedAt,
		Version:     gateway.Version,
	}
}
