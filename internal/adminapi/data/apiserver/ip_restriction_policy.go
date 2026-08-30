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

// IPRestrictionPolicyStore 读写 IPRestrictionPolicy 声明式资源。
type IPRestrictionPolicyStore struct {
	client clientset.Interface
}

// NewIPRestrictionPolicyStore 创建 IPRestrictionPolicy Store。
func NewIPRestrictionPolicyStore(client clientset.Interface) *IPRestrictionPolicyStore {
	return &IPRestrictionPolicyStore{client: client}
}

// ListPage 分页返回 IPRestrictionPolicy。
func (s *IPRestrictionPolicyStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.IPRestrictionPolicy], error) {
	policies, err := s.client.GatewayV1().IPRestrictionPolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.IPRestrictionPolicy]{}, listError("IP restriction policies", err)
	}
	return biz.PageResult[resource.IPRestrictionPolicy]{
		Items:      policies.Items,
		NextCursor: policies.Continue,
	}, nil
}

// Get 返回指定 IPRestrictionPolicy。
func (s *IPRestrictionPolicyStore) Get(
	ctx context.Context,
	policyID string,
) (*resource.IPRestrictionPolicy, error) {
	policy, err := s.client.GatewayV1().IPRestrictionPolicies().Get(
		ctx,
		policyID,
		metav1.GetOptions{},
	)
	return policy, resourceError("get", "IP restriction policy", policyID, err)
}

// Create 创建 IPRestrictionPolicy。
func (s *IPRestrictionPolicyStore) Create(
	ctx context.Context,
	policyID string,
	spec resource.IPRestrictionPolicySpec,
) (*resource.IPRestrictionPolicy, error) {
	policy := &resource.IPRestrictionPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindIPRestrictionPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: policyID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().IPRestrictionPolicies().Create(
		ctx,
		policy,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "IP restriction policy", policyID, err)
}

// ReplaceSpec 完整替换 IPRestrictionPolicy 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *IPRestrictionPolicyStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.IPRestrictionPolicy,
	spec resource.IPRestrictionPolicySpec,
) (*resource.IPRestrictionPolicy, error) {
	policyID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().IPRestrictionPolicies(),
		observed,
		func(policy *resource.IPRestrictionPolicy) { policy.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf(
			"replace IP restriction policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return updated, resourceError("replace", "IP restriction policy", policyID, err)
}

// Delete 删除 IPRestrictionPolicy。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *IPRestrictionPolicyStore) Delete(
	ctx context.Context,
	observed *resource.IPRestrictionPolicy,
) error {
	policyID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().IPRestrictionPolicies(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf(
			"delete IP restriction policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return resourceError("delete", "IP restriction policy", policyID, err)
}
