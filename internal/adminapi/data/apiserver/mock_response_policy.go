package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// MockResponsePolicyStore 读写 MockResponsePolicy 声明式资源。
type MockResponsePolicyStore struct {
	client clientset.Interface
}

// NewMockResponsePolicyStore 创建 MockResponsePolicy Store。
func NewMockResponsePolicyStore(client clientset.Interface) *MockResponsePolicyStore {
	return &MockResponsePolicyStore{client: client}
}

// ListPage 分页返回 MockResponsePolicy。
func (s *MockResponsePolicyStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.MockResponsePolicy], error) {
	policies, err := s.client.GatewayV1().MockResponsePolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.MockResponsePolicy]{}, listError("mock response policies", err)
	}
	return biz.PageResult[resource.MockResponsePolicy]{
		Items:      policies.Items,
		NextCursor: policies.Continue,
	}, nil
}

// Get 返回指定 MockResponsePolicy。
func (s *MockResponsePolicyStore) Get(
	ctx context.Context,
	policyID string,
) (*resource.MockResponsePolicy, error) {
	policy, err := s.client.GatewayV1().MockResponsePolicies().Get(
		ctx,
		policyID,
		metav1.GetOptions{},
	)
	return policy, resourceError("get", "mock response policy", policyID, err)
}

// Create 创建 MockResponsePolicy。
func (s *MockResponsePolicyStore) Create(
	ctx context.Context,
	policyID string,
	spec resource.MockResponsePolicySpec,
) (*resource.MockResponsePolicy, error) {
	policy := &resource.MockResponsePolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindMockResponsePolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: policyID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().MockResponsePolicies().Create(
		ctx,
		policy,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "mock response policy", policyID, err)
}

// ReplaceSpec 完整替换 MockResponsePolicy 配置。
func (s *MockResponsePolicyStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.MockResponsePolicy,
	spec resource.MockResponsePolicySpec,
) (*resource.MockResponsePolicy, error) {
	return replaceResourceSpec(
		ctx,
		s.client.GatewayV1().MockResponsePolicies(),
		"mock response policy",
		observed,
		func(policy *resource.MockResponsePolicy) { policy.Spec = spec },
	)
}

// Delete 删除 MockResponsePolicy。
func (s *MockResponsePolicyStore) Delete(
	ctx context.Context,
	observed *resource.MockResponsePolicy,
) error {
	return deleteResource(
		ctx,
		s.client.GatewayV1().MockResponsePolicies(),
		"mock response policy",
		observed,
	)
}
