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

// TokenQuotaPolicyStore 读写 TokenQuotaPolicy 声明式资源。
type TokenQuotaPolicyStore struct {
	client clientset.Interface
}

// NewTokenQuotaPolicyStore 创建 TokenQuotaPolicy Store。
func NewTokenQuotaPolicyStore(client clientset.Interface) *TokenQuotaPolicyStore {
	return &TokenQuotaPolicyStore{client: client}
}

// ListPage 分页返回 TokenQuotaPolicy。
func (s *TokenQuotaPolicyStore) ListPage(
	ctx context.Context,
	page biz.PageRequest,
) (biz.PageResult[resource.TokenQuotaPolicy], error) {
	policies, err := s.client.GatewayV1().TokenQuotaPolicies().List(ctx, listOptions(page))
	if err != nil {
		return biz.PageResult[resource.TokenQuotaPolicy]{}, listError("token quota policies", err)
	}
	return biz.PageResult[resource.TokenQuotaPolicy]{
		Items:      policies.Items,
		NextCursor: policies.Continue,
	}, nil
}

// Get 返回指定 TokenQuotaPolicy。
func (s *TokenQuotaPolicyStore) Get(
	ctx context.Context,
	policyID string,
) (*resource.TokenQuotaPolicy, error) {
	policy, err := s.client.GatewayV1().TokenQuotaPolicies().Get(
		ctx,
		policyID,
		metav1.GetOptions{},
	)
	return policy, resourceError("get", "token quota policy", policyID, err)
}

// Create 创建 TokenQuotaPolicy。
func (s *TokenQuotaPolicyStore) Create(
	ctx context.Context,
	policyID string,
	spec resource.TokenQuotaPolicySpec,
) (*resource.TokenQuotaPolicy, error) {
	policy := &resource.TokenQuotaPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: resource.SchemeGroupVersion.String(),
			Kind:       string(resource.KindTokenQuotaPolicy),
		},
		ObjectMeta: metav1.ObjectMeta{Name: policyID},
		Spec:       spec,
	}
	created, err := s.client.GatewayV1().TokenQuotaPolicies().Create(
		ctx,
		policy,
		metav1.CreateOptions{},
	)
	return created, resourceError("create", "token quota policy", policyID, err)
}

// ReplaceSpec 完整替换 TokenQuotaPolicy 配置。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *TokenQuotaPolicyStore) ReplaceSpec(
	ctx context.Context,
	observed *resource.TokenQuotaPolicy,
	spec resource.TokenQuotaPolicySpec,
) (*resource.TokenQuotaPolicy, error) {
	policyID := observed.Name
	updated, err := updateResource(
		ctx,
		s.client.GatewayV1().TokenQuotaPolicies(),
		observed,
		func(policy *resource.TokenQuotaPolicy) { policy.Spec = spec },
	)
	if apierrors.IsConflict(err) {
		return nil, fmt.Errorf(
			"replace token quota policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return updated, resourceError("replace", "token quota policy", policyID, err)
}

// Delete 删除 TokenQuotaPolicy。
// 底层资源版本冲突时，仅当 UID 和配置版本仍与初次读取的资源一致时重试。
func (s *TokenQuotaPolicyStore) Delete(
	ctx context.Context,
	observed *resource.TokenQuotaPolicy,
) error {
	policyID := observed.Name
	err := deleteResource(ctx, s.client.GatewayV1().TokenQuotaPolicies(), observed)
	if apierrors.IsConflict(err) {
		return fmt.Errorf(
			"delete token quota policy %q after conflict retries: %w",
			policyID,
			err,
		)
	}
	return resourceError("delete", "token quota policy", policyID, err)
}
