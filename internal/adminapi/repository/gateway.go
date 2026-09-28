package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lgc202/ingate/internal/adminapi/domain"
	"github.com/lgc202/ingate/internal/adminapi/repository/mysql"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayclient "github.com/lgc202/ingate/pkg/generated/clientset/versioned/typed/gateway/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Gateway 将写入交给 API Server，将查询交给 MySQL 投影。
type Gateway struct {
	client gatewayclient.GatewayInterface
	store  *mysql.Store
}

// NewGateway 创建 Gateway Repository。
func NewGateway(client gatewayclient.GatewayInterface, store *mysql.Store) *Gateway {
	return &Gateway{client: client, store: store}
}

// Create 创建 Gateway，并返回 API Server 保存后的业务状态。
func (r *Gateway) Create(ctx context.Context, gateway domain.Gateway) (domain.Gateway, error) {
	created, err := r.client.Create(ctx, gatewayToAPI(gateway), metav1.CreateOptions{})
	if err != nil {
		return domain.Gateway{}, translateResourceError(err)
	}

	return gatewayFromAPI(created), nil
}

// Get 从 MySQL 查询模型读取指定 Gateway。
func (r *Gateway) Get(ctx context.Context, name string) (domain.Gateway, error) {
	gateway, err := r.store.GetGateway(ctx, name)
	if err != nil {
		if errors.Is(err, mysql.ErrNotFound) {
			return domain.Gateway{}, fmt.Errorf("%w: %w", ErrNotFound, err)
		}
		return domain.Gateway{}, err
	}

	return gatewayFromMySQL(gateway), nil
}

// List 从 MySQL 查询模型按页码读取 Gateway。
func (r *Gateway) List(ctx context.Context, size, number int32) ([]domain.Gateway, int64, error) {
	result, err := r.store.ListGateways(ctx, size, number)
	if err != nil {
		return nil, 0, err
	}

	gateways := make([]domain.Gateway, len(result.Items))
	for i, gateway := range result.Items {
		gateways[i] = gatewayFromMySQL(gateway)
	}

	return gateways, result.TotalCount, nil
}

// Update 使用 API Server 当前对象核对版本，并替换 Gateway 的可编辑配置。
func (r *Gateway) Update(ctx context.Context, gateway domain.Gateway) (domain.Gateway, error) {
	current, err := r.client.Get(ctx, gateway.Name, metav1.GetOptions{})
	if err != nil {
		return domain.Gateway{}, translateResourceError(err)
	}

	if current.ResourceVersion != gateway.Version {
		return domain.Gateway{}, ErrConflict
	}

	current.Spec = gatewayToAPI(gateway).Spec
	updated, err := r.client.Update(ctx, current, metav1.UpdateOptions{})
	if err != nil {
		return domain.Gateway{}, translateResourceError(err)
	}

	return gatewayFromAPI(updated), nil
}

// Delete 删除指定 Gateway。
func (r *Gateway) Delete(ctx context.Context, name string) error {
	return translateResourceError(r.client.Delete(ctx, name, metav1.DeleteOptions{}))
}

func gatewayToAPI(gateway domain.Gateway) *gatewayv1alpha1.Gateway {
	listeners := make([]gatewayv1alpha1.Listener, len(gateway.Listeners))
	for i, listener := range gateway.Listeners {
		listeners[i] = gatewayv1alpha1.Listener{
			Name:     listener.Name,
			Protocol: gatewayv1alpha1.Protocol(listener.Protocol),
			Port:     listener.Port,
			Hostname: listener.Hostname,
		}
	}

	return &gatewayv1alpha1.Gateway{
		ObjectMeta: metav1.ObjectMeta{Name: gateway.Name},
		Spec: gatewayv1alpha1.GatewaySpec{
			DisplayName: gateway.DisplayName,
			Listeners:   listeners,
		},
	}
}

func gatewayFromAPI(gateway *gatewayv1alpha1.Gateway) domain.Gateway {
	listeners := make([]domain.Listener, len(gateway.Spec.Listeners))
	for i, listener := range gateway.Spec.Listeners {
		listeners[i] = domain.Listener{
			Name:     listener.Name,
			Protocol: string(listener.Protocol),
			Port:     listener.Port,
			Hostname: listener.Hostname,
		}
	}

	return domain.Gateway{
		Name:        gateway.Name,
		DisplayName: gateway.Spec.DisplayName,
		Listeners:   listeners,
		CreateTime:  gateway.CreationTimestamp.Time,
		Version:     gateway.ResourceVersion,
	}
}

func gatewayFromMySQL(gateway mysql.Gateway) domain.Gateway {
	listeners := make([]domain.Listener, len(gateway.Listeners))
	for i, listener := range gateway.Listeners {
		listeners[i] = domain.Listener{
			Name:     listener.Name,
			Protocol: listener.Protocol,
			Port:     int32(listener.Port),
			Hostname: listener.Hostname,
		}
	}

	return domain.Gateway{
		Name:        gateway.Name,
		DisplayName: gateway.DisplayName,
		Listeners:   listeners,
		CreateTime:  gateway.CreatedAt,
		Version:     gateway.ResourceVersion,
	}
}
