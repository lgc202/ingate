package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// RateLimitPolicyRepository 读写 RateLimitPolicy 声明式资源
type RateLimitPolicyRepository struct {
	client clientset.Interface
}

// NewRateLimitPolicyRepository 创建 RateLimitPolicy Repository
func NewRateLimitPolicyRepository(client clientset.Interface) *RateLimitPolicyRepository {
	return &RateLimitPolicyRepository{client: client}
}

// ListPage 分页查询 RateLimitPolicy 列表
func (r *RateLimitPolicyRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.RateLimitPolicy], error) {
	policies, err := r.client.GatewayV1().RateLimitPolicies().List(ctx, pageOptions(page))
	if err != nil {
		return biz.PageResult[resource.RateLimitPolicy]{}, pageError("rate limit policies", err)
	}
	return biz.PageResult[resource.RateLimitPolicy]{Items: policies.Items, NextToken: policies.Continue}, nil
}

// Get 查询单个 RateLimitPolicy
func (r *RateLimitPolicyRepository) Get(ctx context.Context, name string) (*resource.RateLimitPolicy, error) {
	policy, err := r.client.GatewayV1().RateLimitPolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "rate limit policy", name, err)
}

// Create 创建 RateLimitPolicy
func (r *RateLimitPolicyRepository) Create(ctx context.Context, id string, spec resource.RateLimitPolicySpec) error {
	policy := &resource.RateLimitPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindRateLimitPolicy)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	_, err := r.client.GatewayV1().RateLimitPolicies().Create(ctx, policy, metav1.CreateOptions{})
	return resourceError("create", "rate limit policy", id, err)
}

// Update 更新 RateLimitPolicy，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *RateLimitPolicyRepository) Update(ctx context.Context, id string, generation int64, spec resource.RateLimitPolicySpec) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().RateLimitPolicies().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		_, err = r.client.GatewayV1().RateLimitPolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return resourceError("update", "rate limit policy", id, err)
}

// Delete 删除 RateLimitPolicy
func (r *RateLimitPolicyRepository) Delete(ctx context.Context, name string) error {
	err := r.client.GatewayV1().RateLimitPolicies().Delete(ctx, name, metav1.DeleteOptions{})
	return resourceError("delete", "rate limit policy", name, err)
}
