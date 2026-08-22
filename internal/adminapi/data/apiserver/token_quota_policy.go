package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// TokenQuotaPolicyRepository 读写 TokenQuotaPolicy 声明式资源
type TokenQuotaPolicyRepository struct {
	client clientset.Interface
}

// NewTokenQuotaPolicyRepository 创建 TokenQuotaPolicy Repository
func NewTokenQuotaPolicyRepository(client clientset.Interface) *TokenQuotaPolicyRepository {
	return &TokenQuotaPolicyRepository{client: client}
}

// ListPage 分页查询 TokenQuotaPolicy 列表
func (r *TokenQuotaPolicyRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.TokenQuotaPolicy], error) {
	policies, err := r.client.GatewayV1().TokenQuotaPolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.TokenQuotaPolicy]{}, listError("token quota policies", err)
	}
	return biz.PageResult[resource.TokenQuotaPolicy]{Items: policies.Items, NextCursor: policies.Continue}, nil
}

// Get 查询单个 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Get(ctx context.Context, name string) (*resource.TokenQuotaPolicy, error) {
	policy, err := r.client.GatewayV1().TokenQuotaPolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "token quota policy", name, err)
}

// Create 创建 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Create(
	ctx context.Context,
	name string,
	spec resource.TokenQuotaPolicySpec,
) (*resource.TokenQuotaPolicy, error) {
	policy := &resource.TokenQuotaPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindTokenQuotaPolicy)},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().TokenQuotaPolicies().Create(ctx, policy, metav1.CreateOptions{})
	return created, resourceError("create", "token quota policy", name, err)
}

// Update 更新 TokenQuotaPolicy，并只重试状态写入导致的 ResourceVersion 冲突
func (r *TokenQuotaPolicyRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.TokenQuotaPolicySpec,
) (*resource.TokenQuotaPolicy, error) {
	var updated *resource.TokenQuotaPolicy
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().TokenQuotaPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().TokenQuotaPolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "token quota policy", name, err)
}

// Delete 删除 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().TokenQuotaPolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().TokenQuotaPolicies().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "token quota policy", name, err)
}
