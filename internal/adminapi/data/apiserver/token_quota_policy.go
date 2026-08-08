package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// TokenQuotaPolicyRepository 读写 TokenQuotaPolicy 声明式资源
type TokenQuotaPolicyRepository struct {
	client clientset.Interface
}

// NewTokenQuotaPolicy 创建 TokenQuotaPolicy Repository
func NewTokenQuotaPolicy(client clientset.Interface) *TokenQuotaPolicyRepository {
	return &TokenQuotaPolicyRepository{client: client}
}

// List 查询 TokenQuotaPolicy 列表
func (r *TokenQuotaPolicyRepository) List(ctx context.Context) (*resource.TokenQuotaPolicyList, error) {
	return r.client.GatewayV1().TokenQuotaPolicies().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Get(ctx context.Context, name string) (*resource.TokenQuotaPolicy, error) {
	return r.client.GatewayV1().TokenQuotaPolicies().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Create(ctx context.Context, policy *resource.TokenQuotaPolicy) (*resource.TokenQuotaPolicy, error) {
	return r.client.GatewayV1().TokenQuotaPolicies().Create(ctx, policy, metav1.CreateOptions{})
}

// Update 更新 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Update(ctx context.Context, policy *resource.TokenQuotaPolicy) (*resource.TokenQuotaPolicy, error) {
	return r.client.GatewayV1().TokenQuotaPolicies().Update(ctx, policy, metav1.UpdateOptions{})
}

// Delete 删除 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().TokenQuotaPolicies().Delete(ctx, name, metav1.DeleteOptions{})
}
