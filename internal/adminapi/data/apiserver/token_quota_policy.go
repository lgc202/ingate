package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// TokenQuotaPolicyRepository 读写 TokenQuotaPolicy 声明式资源
type TokenQuotaPolicyRepository struct {
	client clientset.Interface
}

// NewTokenQuotaPolicyRepository 创建 TokenQuotaPolicy Repository
func NewTokenQuotaPolicyRepository(client clientset.Interface) *TokenQuotaPolicyRepository {
	return &TokenQuotaPolicyRepository{client: client}
}

// List 查询 TokenQuotaPolicy 列表
func (r *TokenQuotaPolicyRepository) List(ctx context.Context) ([]resource.TokenQuotaPolicy, error) {
	policies, err := r.client.GatewayV1().TokenQuotaPolicies().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, resourceError("list", "token quota policies", "", err)
	}
	return policies.Items, nil
}

// Get 查询单个 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Get(ctx context.Context, name string) (*resource.TokenQuotaPolicy, error) {
	policy, err := r.client.GatewayV1().TokenQuotaPolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "token quota policy", name, err)
}

// Create 创建 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Create(ctx context.Context, id string, spec resource.TokenQuotaPolicySpec) error {
	policy := &resource.TokenQuotaPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindTokenQuotaPolicy)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	_, err := r.client.GatewayV1().TokenQuotaPolicies().Create(ctx, policy, metav1.CreateOptions{})
	return resourceError("create", "token quota policy", id, err)
}

// Update 更新 TokenQuotaPolicy，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *TokenQuotaPolicyRepository) Update(ctx context.Context, id string, generation int64, spec resource.TokenQuotaPolicySpec) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().TokenQuotaPolicies().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		_, err = r.client.GatewayV1().TokenQuotaPolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return resourceError("update", "token quota policy", id, err)
}

// Delete 删除 TokenQuotaPolicy
func (r *TokenQuotaPolicyRepository) Delete(ctx context.Context, name string) error {
	err := r.client.GatewayV1().TokenQuotaPolicies().Delete(ctx, name, metav1.DeleteOptions{})
	return resourceError("delete", "token quota policy", name, err)
}
