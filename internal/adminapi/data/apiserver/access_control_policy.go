package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/pkg/generated/clientset/versioned"
)

// AccessControlPolicyRepository 读写 AccessControlPolicy 声明式资源
type AccessControlPolicyRepository struct {
	client clientset.Interface
}

// NewAccessControlPolicyRepository 创建 AccessControlPolicy Repository
func NewAccessControlPolicyRepository(client clientset.Interface) *AccessControlPolicyRepository {
	return &AccessControlPolicyRepository{client: client}
}

// ListPage 分页查询 AccessControlPolicy 列表
func (r *AccessControlPolicyRepository) ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.AccessControlPolicy], error) {
	policies, err := r.client.GatewayV1().AccessControlPolicies().List(ctx, pageOptions(page))
	if err != nil {
		return biz.PageResult[resource.AccessControlPolicy]{}, pageError("access control policies", err)
	}
	return biz.PageResult[resource.AccessControlPolicy]{Items: policies.Items, NextToken: policies.Continue}, nil
}

// Get 查询单个 AccessControlPolicy
func (r *AccessControlPolicyRepository) Get(ctx context.Context, name string) (*resource.AccessControlPolicy, error) {
	policy, err := r.client.GatewayV1().AccessControlPolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "access control policy", name, err)
}

// Create 创建 AccessControlPolicy
func (r *AccessControlPolicyRepository) Create(ctx context.Context, id string, spec resource.AccessControlPolicySpec) error {
	policy := &resource.AccessControlPolicy{
		TypeMeta:   metav1.TypeMeta{APIVersion: resource.SchemeGroupVersion.String(), Kind: string(resource.KindAccessControlPolicy)},
		ObjectMeta: metav1.ObjectMeta{Name: id},
		Spec:       spec,
	}
	_, err := r.client.GatewayV1().AccessControlPolicies().Create(ctx, policy, metav1.CreateOptions{})
	return resourceError("create", "access control policy", id, err)
}

// Update 更新 AccessControlPolicy，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *AccessControlPolicyRepository) Update(ctx context.Context, id string, generation int64, spec resource.AccessControlPolicySpec) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().AccessControlPolicies().Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		_, err = r.client.GatewayV1().AccessControlPolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return resourceError("update", "access control policy", id, err)
}

// Delete 删除 AccessControlPolicy
func (r *AccessControlPolicyRepository) Delete(ctx context.Context, name string) error {
	err := r.client.GatewayV1().AccessControlPolicies().Delete(ctx, name, metav1.DeleteOptions{})
	return resourceError("delete", "access control policy", name, err)
}
