package gateway

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读取 Gateway 资源
type Store struct {
	client clientset.Interface
}

// New 创建 Gateway store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 Gateway 列表
func (s *Store) List(ctx context.Context) (*resource.GatewayList, error) {
	return s.client.GatewayV1().Gateways().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 Gateway
func (s *Store) Get(ctx context.Context, name string) (*resource.Gateway, error) {
	return s.client.GatewayV1().Gateways().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 Gateway
func (s *Store) Create(ctx context.Context, gateway *resource.Gateway) (*resource.Gateway, error) {
	return s.client.GatewayV1().Gateways().Create(ctx, gateway, metav1.CreateOptions{})
}

// Update 更新 Gateway
func (s *Store) Update(ctx context.Context, gateway *resource.Gateway) (*resource.Gateway, error) {
	return s.client.GatewayV1().Gateways().Update(ctx, gateway, metav1.UpdateOptions{})
}

// Delete 删除 Gateway
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().Gateways().Delete(ctx, name, metav1.DeleteOptions{})
}
