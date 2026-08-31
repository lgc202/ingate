package apiserver

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	clientset "github.com/lgc202/ingate/internal/pkg/generated/clientset/versioned"
)

// HeaderTransformationPolicyStore 读写 HeaderTransformationPolicy 声明式资源。
type HeaderTransformationPolicyStore struct {
	client clientset.Interface
}

// NewHeaderTransformationPolicyStore 创建 HeaderTransformationPolicy Store。
func NewHeaderTransformationPolicyStore(
	client clientset.Interface,
) *HeaderTransformationPolicyStore {
	return &HeaderTransformationPolicyStore{client: client}
}

// ListPage 分页返回 HeaderTransformationPolicy。
func (s *HeaderTransformationPolicyStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.HeaderTransformationPolicy], error) {
	policies, err := s.client.GatewayV1().HeaderTransformationPolicies().List(
		ctx,
		listOptions(page),
	)
	if err != nil {
		return biz.PageResult[resource.HeaderTransformationPolicy]{}, listError(
			"header transformation policies",
			err,
		)
	}
	return biz.PageResult[resource.HeaderTransformationPolicy]{
		Items:      policies.Items,
		NextCursor: policies.Continue,
	}, nil
}

// Get 返回指定 HeaderTransformationPolicy。
func (s *HeaderTransformationPolicyStore) Get(
	ctx context.Context,
	policyID string,
) (*resource.HeaderTransformationPolicy, error) {
	policy, err := s.client.GatewayV1().HeaderTransformationPolicies().Get(
		ctx,
		policyID,
		metav1.GetOptions{},
	)
	return policy, resourceError("get", "header transformation policy", policyID, err)
}

// Create 创建 HeaderTransformationPolicy。
func (s *HeaderTransformationPolicyStore) Create(
	ctx context.Context,
	policyID string,
	spec resource.HeaderTransformationPolicySpec,
) (*resource.HeaderTransformationPolicy, error) {
	policy := &resource.HeaderTransformationPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindHeaderTransformationPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: policyID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().HeaderTransformationPolicies().Create(
		ctx,
		policy,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "header transformation policy", policyID, err)
}

// ReplaceSpec 完整替换 HeaderTransformationPolicy 配置。
func (s *HeaderTransformationPolicyStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.HeaderTransformationPolicy,
	spec resource.HeaderTransformationPolicySpec,
) (*resource.HeaderTransformationPolicy, error) {
	return replaceResourceSpec(
		ctx,
		s.client.GatewayV1().HeaderTransformationPolicies(),
		"header transformation policy",
		observed,
		func(policy *resource.HeaderTransformationPolicy) { policy.Spec = spec },
	)
}

// Delete 删除 HeaderTransformationPolicy。
func (s *HeaderTransformationPolicyStore) Delete(
	ctx context.Context,
	observed *resource.HeaderTransformationPolicy,
) error {
	return deleteResource(
		ctx,
		s.client.GatewayV1().HeaderTransformationPolicies(),
		"header transformation policy",
		observed,
	)
}
