package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// AccessControlPolicyRepository 读写 AccessControlPolicy 声明式资源
type AccessControlPolicyRepository struct {
	client clientset.Interface
}

// NewAccessControlPolicy 创建 AccessControlPolicy Repository
func NewAccessControlPolicy(client clientset.Interface) *AccessControlPolicyRepository {
	return &AccessControlPolicyRepository{client: client}
}

// List 查询 AccessControlPolicy 列表
func (r *AccessControlPolicyRepository) List(ctx context.Context) (*resource.AccessControlPolicyList, error) {
	return r.client.GatewayV1().AccessControlPolicies().List(ctx, metav1.ListOptions{})
}

// Get 查询单个 AccessControlPolicy
func (r *AccessControlPolicyRepository) Get(ctx context.Context, name string) (*resource.AccessControlPolicy, error) {
	return r.client.GatewayV1().AccessControlPolicies().Get(ctx, name, metav1.GetOptions{})
}

// Create 创建 AccessControlPolicy
func (r *AccessControlPolicyRepository) Create(ctx context.Context, policy *resource.AccessControlPolicy) (*resource.AccessControlPolicy, error) {
	return r.client.GatewayV1().AccessControlPolicies().Create(ctx, policy, metav1.CreateOptions{})
}

// Update 更新 AccessControlPolicy
func (r *AccessControlPolicyRepository) Update(ctx context.Context, policy *resource.AccessControlPolicy) (*resource.AccessControlPolicy, error) {
	return r.client.GatewayV1().AccessControlPolicies().Update(ctx, policy, metav1.UpdateOptions{})
}

// Delete 删除 AccessControlPolicy
func (r *AccessControlPolicyRepository) Delete(ctx context.Context, name string) error {
	return r.client.GatewayV1().AccessControlPolicies().Delete(ctx, name, metav1.DeleteOptions{})
}
