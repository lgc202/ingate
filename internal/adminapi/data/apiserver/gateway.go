package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// GatewayRepository 读写 Gateway 声明式资源
type GatewayRepository struct {
	client clientset.Interface
}

// NewGateway 创建 Gateway Repository
func NewGateway(client clientset.Interface) *GatewayRepository {
	return &GatewayRepository{client: client}
}

// List 查询 Gateway 列表
func (r *GatewayRepository) List(ctx context.Context) (*resource.GatewayList, error) {
	return r.client.GatewayV1().Gateways().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Gateway
func (r *GatewayRepository) Get(ctx context.Context, name string) (*resource.Gateway, error) {
	return r.client.GatewayV1().Gateways().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Gateway
func (r *GatewayRepository) Create(ctx context.Context, gateway *resource.Gateway) (*resource.Gateway, error) {
	return r.client.GatewayV1().Gateways().Create(ctx, gateway, metav1.CreateOptions{})
}

// Update 更新 Gateway
func (r *GatewayRepository) Update(ctx context.Context, gateway *resource.Gateway) (*resource.Gateway, error) {
	return r.client.GatewayV1().Gateways().Update(ctx, gateway, metav1.UpdateOptions{})
}

// Delete 删除 Gateway
func (r *GatewayRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().Gateways().Delete(ctx, name, metav1.DeleteOptions{})
}
