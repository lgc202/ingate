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

// RateLimitPolicyStore 读写 RateLimitPolicy 声明式资源。
type RateLimitPolicyStore struct {
	client clientset.Interface
}

// NewRateLimitPolicyStore 创建 RateLimitPolicy Store。
func NewRateLimitPolicyStore(client clientset.Interface) *RateLimitPolicyStore {
	return &RateLimitPolicyStore{client: client}
}

// ListPage 分页返回 RateLimitPolicy。
func (s *RateLimitPolicyStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.RateLimitPolicy], error) {
	policies, err := s.client.GatewayV1().RateLimitPolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.RateLimitPolicy]{}, listError("rate limit policies", err)
	}
	return biz.PageResult[resource.RateLimitPolicy]{
		Items:      policies.Items,
		NextCursor: policies.Continue,
	}, nil
}

// Get 返回指定 RateLimitPolicy。
func (s *RateLimitPolicyStore) Get(
	ctx context.Context,
	policyID string,
) (*resource.RateLimitPolicy, error) {
	policy, err := s.client.GatewayV1().RateLimitPolicies().Get(
		ctx,
		policyID,
		metav1.GetOptions{},
	)
	return policy, resourceError("get", "rate limit policy", policyID, err)
}

// Create 创建 RateLimitPolicy。
func (s *RateLimitPolicyStore) Create(
	ctx context.Context,
	policyID string,
	spec resource.RateLimitPolicySpec,
) (*resource.RateLimitPolicy, error) {
	policy := &resource.RateLimitPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindRateLimitPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: policyID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().RateLimitPolicies().Create(
		ctx,
		policy,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "rate limit policy", policyID, err)
}

// ReplaceSpec 完整替换 RateLimitPolicy 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *RateLimitPolicyStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.RateLimitPolicy,
	spec resource.RateLimitPolicySpec,
) (*resource.RateLimitPolicy, error) {
	policyID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().RateLimitPolicies(),
		observed,
		func(policy *resource.RateLimitPolicy) { policy.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf(
			"replace rate limit policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return updated, resourceError("replace", "rate limit policy", policyID, err)
}

// Delete 删除 RateLimitPolicy。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *RateLimitPolicyStore) Delete(
	ctx context.Context,
	observed *resource.RateLimitPolicy,
) error {
	policyID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().RateLimitPolicies(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf(
			"delete rate limit policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return resourceError("delete", "rate limit policy", policyID, err)
}
