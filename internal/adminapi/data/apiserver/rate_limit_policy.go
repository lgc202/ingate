package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// RateLimitPolicyRepository 读写 RateLimitPolicy 声明式资源
type RateLimitPolicyRepository struct {
	client clientset.Interface
}

// NewRateLimitPolicy 创建 RateLimitPolicy Repository
func NewRateLimitPolicy(client clientset.Interface) *RateLimitPolicyRepository {
	return &RateLimitPolicyRepository{client: client}
}

// List 查询 RateLimitPolicy 列表
func (r *RateLimitPolicyRepository) List(ctx context.Context) (*resource.RateLimitPolicyList, error) {
	return r.client.GatewayV1().RateLimitPolicies().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 RateLimitPolicy
func (r *RateLimitPolicyRepository) Get(ctx context.Context, name string) (*resource.RateLimitPolicy, error) {
	return r.client.GatewayV1().RateLimitPolicies().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 RateLimitPolicy
func (r *RateLimitPolicyRepository) Create(ctx context.Context, policy *resource.RateLimitPolicy) (*resource.RateLimitPolicy, error) {
	return r.client.GatewayV1().RateLimitPolicies().Create(ctx, policy, metav1.CreateOptions{})
}

// Update 更新 RateLimitPolicy
func (r *RateLimitPolicyRepository) Update(ctx context.Context, policy *resource.RateLimitPolicy) (*resource.RateLimitPolicy, error) {
	return r.client.GatewayV1().RateLimitPolicies().Update(ctx, policy, metav1.UpdateOptions{})
}

// Delete 删除 RateLimitPolicy
func (r *RateLimitPolicyRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().RateLimitPolicies().Delete(ctx, name, metav1.DeleteOptions{})
}
