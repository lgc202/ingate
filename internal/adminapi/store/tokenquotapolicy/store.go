package tokenquotapolicy

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读写 TokenQuotaPolicy 资源
type Store struct {
	client clientset.Interface
}

// New 创建 TokenQuotaPolicy store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 TokenQuotaPolicy 列表
func (s *Store) List(ctx context.Context) (*resource.TokenQuotaPolicyList, error) {
	return s.client.GatewayV1().TokenQuotaPolicies().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 TokenQuotaPolicy
func (s *Store) Get(ctx context.Context, name string) (*resource.TokenQuotaPolicy, error) {
	return s.client.GatewayV1().TokenQuotaPolicies().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 TokenQuotaPolicy
func (s *Store) Create(ctx context.Context, policy *resource.TokenQuotaPolicy) (*resource.TokenQuotaPolicy, error) {
	return s.client.GatewayV1().TokenQuotaPolicies().Create(ctx, policy, metav1.CreateOptions{})
}

// Update 更新 TokenQuotaPolicy
func (s *Store) Update(ctx context.Context, policy *resource.TokenQuotaPolicy) (*resource.TokenQuotaPolicy, error) {
	return s.client.GatewayV1().TokenQuotaPolicies().Update(ctx, policy, metav1.UpdateOptions{})
}

// Delete 删除 TokenQuotaPolicy
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().TokenQuotaPolicies().Delete(ctx, name, metav1.DeleteOptions{})
}
