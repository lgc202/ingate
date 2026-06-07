package policybinding

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读写 PolicyBinding 资源
type Store struct {
	client clientset.Interface
}

// New 创建 PolicyBinding store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 PolicyBinding 列表
func (s *Store) List(ctx context.Context) (*resource.PolicyBindingList, error) {
	return s.client.GatewayV1().PolicyBindings().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 PolicyBinding
func (s *Store) Get(ctx context.Context, name string) (*resource.PolicyBinding, error) {
	return s.client.GatewayV1().PolicyBindings().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 PolicyBinding
func (s *Store) Create(ctx context.Context, binding *resource.PolicyBinding) (*resource.PolicyBinding, error) {
	return s.client.GatewayV1().PolicyBindings().Create(ctx, binding, metav1.CreateOptions{})
}

// Update 更新 PolicyBinding
func (s *Store) Update(ctx context.Context, binding *resource.PolicyBinding) (*resource.PolicyBinding, error) {
	return s.client.GatewayV1().PolicyBindings().Update(ctx, binding, metav1.UpdateOptions{})
}

// Delete 删除 PolicyBinding
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().PolicyBindings().Delete(ctx, name, metav1.DeleteOptions{})
}
