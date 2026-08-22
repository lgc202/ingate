package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// HeaderTransformationPolicyRepository 读写 HeaderTransformationPolicy 声明式资源
type HeaderTransformationPolicyRepository struct {
	client clientset.Interface
}

// NewHeaderTransformationPolicyRepository 创建 HeaderTransformationPolicy Repository
func NewHeaderTransformationPolicyRepository(client clientset.Interface) *HeaderTransformationPolicyRepository {
	return &HeaderTransformationPolicyRepository{client: client}
}

// ListPage 分页查询 HeaderTransformationPolicy 列表
func (r *HeaderTransformationPolicyRepository) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.HeaderTransformationPolicy], error) {
	policies, err := r.client.GatewayV1().HeaderTransformationPolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.HeaderTransformationPolicy]{}, listError("header transformation policies", err)
	}
	return biz.PageResult[resource.HeaderTransformationPolicy]{Items: policies.Items, NextCursor: policies.Continue}, nil
}

// Get 查询单个 HeaderTransformationPolicy
func (r *HeaderTransformationPolicyRepository) Get(
	ctx context.Context,
	name string,
) (*resource.HeaderTransformationPolicy, error) {
	policy, err := r.client.GatewayV1().HeaderTransformationPolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "header transformation policy", name, err)
}

// Create 创建 HeaderTransformationPolicy
func (r *HeaderTransformationPolicyRepository) Create(
	ctx context.Context,
	name string,
	spec resource.HeaderTransformationPolicySpec,
) (*resource.HeaderTransformationPolicy, error) {
	policy := &resource.HeaderTransformationPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindHeaderTransformationPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().HeaderTransformationPolicies().Create(ctx, policy, metav1.CreateOptions{})
	return created, resourceError("create", "header transformation policy", name, err)
}

// Update 更新 HeaderTransformationPolicy，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *HeaderTransformationPolicyRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.HeaderTransformationPolicySpec,
) (*resource.HeaderTransformationPolicy, error) {
	var updated *resource.HeaderTransformationPolicy
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().HeaderTransformationPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().HeaderTransformationPolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "header transformation policy", name, err)
}

// Delete 删除 HeaderTransformationPolicy
func (r *HeaderTransformationPolicyRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().HeaderTransformationPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().HeaderTransformationPolicies().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "header transformation policy", name, err)
}
