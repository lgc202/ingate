package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// IPRestrictionPolicyRepository 读写 IPRestrictionPolicy 声明式资源
type IPRestrictionPolicyRepository struct {
	client clientset.Interface
}

// NewIPRestrictionPolicyRepository 创建 IPRestrictionPolicy Repository
func NewIPRestrictionPolicyRepository(client clientset.Interface) *IPRestrictionPolicyRepository {
	return &IPRestrictionPolicyRepository{client: client}
}

// ListPage 分页查询 IPRestrictionPolicy 列表
func (r *IPRestrictionPolicyRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.IPRestrictionPolicy], error) {
	policies, err := r.client.GatewayV1().IPRestrictionPolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.IPRestrictionPolicy]{}, listError("IP restriction policies", err)
	}
	return biz.PageResult[resource.IPRestrictionPolicy]{Items: policies.Items, NextCursor: policies.Continue}, nil
}

// Get 查询单个 IPRestrictionPolicy
func (r *IPRestrictionPolicyRepository) Get(ctx context.Context, name string) (*resource.IPRestrictionPolicy, error) {
	policy, err := r.client.GatewayV1().IPRestrictionPolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "IP restriction policy", name, err)
}

// Create 创建 IPRestrictionPolicy
func (r *IPRestrictionPolicyRepository) Create(
	ctx context.Context,
	name string,
	spec resource.IPRestrictionPolicySpec,
) (*resource.IPRestrictionPolicy, error) {
	policy := &resource.IPRestrictionPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindIPRestrictionPolicy)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().IPRestrictionPolicies().Create(ctx, policy, metav1.CreateOptions{})
	return created, resourceError("create", "IP restriction policy", name, err)
}

// Update 更新 IPRestrictionPolicy，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *IPRestrictionPolicyRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.IPRestrictionPolicySpec,
) (*resource.IPRestrictionPolicy, error) {
	var updated *resource.IPRestrictionPolicy
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().IPRestrictionPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().IPRestrictionPolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "IP restriction policy", name, err)
}

// Delete 删除 IPRestrictionPolicy
func (r *IPRestrictionPolicyRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().IPRestrictionPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().IPRestrictionPolicies().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "IP restriction policy", name, err)
}
