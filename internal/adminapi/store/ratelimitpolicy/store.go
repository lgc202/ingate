package ratelimitpolicy

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// Store 读写 RateLimitPolicy 资源
type Store struct {
	client clientset.Interface
}

// New 创建 RateLimitPolicy store
func New(client clientset.Interface) *Store {
	return &Store{client: client}
}

// List 查询 RateLimitPolicy 列表
func (s *Store) List(ctx context.Context) (*resource.RateLimitPolicyList, error) {
	return s.client.GatewayV1().RateLimitPolicies().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 RateLimitPolicy
func (s *Store) Get(ctx context.Context, name string) (*resource.RateLimitPolicy, error) {
	return s.client.GatewayV1().RateLimitPolicies().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 RateLimitPolicy
func (s *Store) Create(ctx context.Context, policy *resource.RateLimitPolicy) (*resource.RateLimitPolicy, error) {
	return s.client.GatewayV1().RateLimitPolicies().Create(ctx, policy, metav1.CreateOptions{})
}

// Update 更新 RateLimitPolicy
func (s *Store) Update(ctx context.Context, policy *resource.RateLimitPolicy) (*resource.RateLimitPolicy, error) {
	return s.client.GatewayV1().RateLimitPolicies().Update(ctx, policy, metav1.UpdateOptions{})
}

// Delete 删除 RateLimitPolicy
func (s *Store) Delete(ctx context.Context, name string) error {
	return s.client.GatewayV1().RateLimitPolicies().Delete(ctx, name, metav1.DeleteOptions{})
}
