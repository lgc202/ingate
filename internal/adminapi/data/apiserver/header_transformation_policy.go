package apiserver

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *HeaderTransformationPolicyStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.HeaderTransformationPolicy,
	spec resource.HeaderTransformationPolicySpec,
) (*resource.HeaderTransformationPolicy, error) {
	policyID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().HeaderTransformationPolicies(),
		observed,
		func(policy *resource.HeaderTransformationPolicy) { policy.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf(
			"replace header transformation policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return updated, resourceError("replace", "header transformation policy", policyID, err)
}

// Delete 删除 HeaderTransformationPolicy。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *HeaderTransformationPolicyStore) Delete(
	ctx context.Context,
	observed *resource.HeaderTransformationPolicy,
) error {
	policyID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().HeaderTransformationPolicies(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf(
			"delete header transformation policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return resourceError("delete", "header transformation policy", policyID, err)
}
