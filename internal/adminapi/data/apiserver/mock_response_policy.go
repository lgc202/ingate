package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// MockResponsePolicyRepository 读写 MockResponsePolicy 声明式资源
type MockResponsePolicyRepository struct {
	client clientset.Interface
}

// NewMockResponsePolicyRepository 创建 MockResponsePolicy Repository
func NewMockResponsePolicyRepository(client clientset.Interface) *MockResponsePolicyRepository {
	return &MockResponsePolicyRepository{client: client}
}

// ListPage 分页查询 MockResponsePolicy 列表
func (r *MockResponsePolicyRepository) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.MockResponsePolicy], error) {
	policies, err := r.client.GatewayV1().MockResponsePolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.MockResponsePolicy]{}, listError("mock response policies", err)
	}
	return biz.PageResult[resource.MockResponsePolicy]{Items: policies.Items, NextCursor: policies.Continue}, nil
}

// Get 查询单个 MockResponsePolicy
func (r *MockResponsePolicyRepository) Get(ctx context.Context, name string) (*resource.MockResponsePolicy, error) {
	policy, err := r.client.GatewayV1().MockResponsePolicies().Get(ctx, name, metav1.GetOptions{})
	return policy, resourceError("get", "mock response policy", name, err)
}

// Create 创建 MockResponsePolicy
func (r *MockResponsePolicyRepository) Create(
	ctx context.Context,
	name string,
	spec resource.MockResponsePolicySpec,
) (*resource.MockResponsePolicy, error) {
	policy := &resource.MockResponsePolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindMockResponsePolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	created, err := r.client.GatewayV1().MockResponsePolicies().Create(ctx, policy, metav1.CreateOptions{})
	return created, resourceError("create", "mock response policy", name, err)
}

// Update 更新 MockResponsePolicy，并只重试 Controller 写 status 导致的 ResourceVersion 冲突
func (r *MockResponsePolicyRepository) Update(
	ctx context.Context,
	name string,
	generation int64,
	spec resource.MockResponsePolicySpec,
) (*resource.MockResponsePolicy, error) {
	var updated *resource.MockResponsePolicy
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().MockResponsePolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		current.Spec = spec
		updated, err = r.client.GatewayV1().MockResponsePolicies().Update(ctx, current, metav1.UpdateOptions{})
		return err
	})
	return updated, resourceError("update", "mock response policy", name, err)
}

// Delete 删除 MockResponsePolicy
func (r *MockResponsePolicyRepository) Delete(ctx context.Context, name string, generation int64) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := r.client.GatewayV1().MockResponsePolicies().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if current.Generation != generation {
			return biz.ErrResourceVersionConflict
		}
		resourceVersion := current.ResourceVersion
		return r.client.GatewayV1().MockResponsePolicies().Delete(ctx, name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &resourceVersion},
		})
	})
	return resourceError("delete", "mock response policy", name, err)
}
