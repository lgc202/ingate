// Package tokenquota 处理模型 Token 额度的管理规则和资源协作
package tokenquota

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Repository 定义 Token 额度策略管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.TokenQuotaPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.TokenQuotaPolicy, error)
	Create(ctx context.Context, policyID string, spec resource.TokenQuotaPolicySpec) (*resource.TokenQuotaPolicy, error)
	Update(ctx context.Context, policyID string, generation int64, spec resource.TokenQuotaPolicySpec) (*resource.TokenQuotaPolicy, error)
	Delete(ctx context.Context, policyID string, generation int64) error
}

// CallerRepository 提供额度目标解析和当前用量查询需要的 Caller 读取能力
type CallerRepository interface {
	biz.CallerLister
	Get(ctx context.Context, callerID string) (*resource.Caller, error)
}

// Service 协调 TokenQuotaPolicy 的调用方解析、校验和持久化
type Service struct {
	repository Repository
	callers    CallerRepository
	usage      UsageReader
	targets    *biz.PolicyTargetResolver
}

// PolicyPage 保存一页策略及其调用方展示名称
type PolicyPage struct {
	Policies    []resource.TokenQuotaPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单个策略及其调用方展示名称
type PolicyView struct {
	Policy      *resource.TokenQuotaPolicy
	TargetNames biz.PolicyTargetNames
}

// NewService 创建 TokenQuotaPolicy 业务服务
func NewService(repository Repository, callers CallerRepository, usage UsageReader) *Service {
	return &Service{
		repository: repository,
		callers:    callers,
		usage:      usage,
		targets:    biz.NewCallerPolicyTargetResolver(callers),
	}
}

// List 查询 TokenQuotaPolicy 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest, filter biz.ResourceFilter) (PolicyPage, error) {
	result, err := biz.FilterPage(ctx, page, s.repository.ListPage, func(policy resource.TokenQuotaPolicy) bool {
		status := biz.PolicyStatus(policy.Generation, policy.Spec.Enabled, len(policy.Spec.TargetRefs), policy.Status.Conditions)
		return filter.Match(policy.Spec.DisplayName, policy.Spec.Enabled, status)
	})
	if err != nil {
		return PolicyPage{}, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return PolicyPage{}, err
	}
	return PolicyPage{Policies: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor}, nil
}

// Get 查询单个 TokenQuotaPolicy
func (s *Service) Get(ctx context.Context, policyID string) (PolicyView, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 TokenQuotaPolicy
func (s *Service) Create(ctx context.Context, spec resource.TokenQuotaPolicySpec) (PolicyView, error) {
	if err := s.ensureDisplayNameAvailable(ctx, "", spec.DisplayName); err != nil {
		return PolicyView{}, err
	}
	targetNames, err := s.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}
	policy, err := s.repository.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Update 使用配置版本乐观更新 TokenQuotaPolicy
func (s *Service) Update(
	ctx context.Context,
	policyID string,
	version int64,
	spec resource.TokenQuotaPolicySpec,
) (PolicyView, error) {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}
	if version != current.Generation {
		return PolicyView{}, versionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := s.ensureDisplayNameAvailable(ctx, policyID, spec.DisplayName); err != nil {
			return PolicyView{}, err
		}
	}
	targetNames, err := s.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}
	updated, err := s.repository.Update(ctx, policyID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return PolicyView{}, versionConflict(current)
		}
		return PolicyView{}, err
	}
	return PolicyView{Policy: updated, TargetNames: targetNames}, nil
}

// Delete 使用配置版本删除 TokenQuotaPolicy
func (s *Service) Delete(ctx context.Context, policyID string, version int64) error {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if version != current.Generation {
		return versionConflict(current)
	}
	if err := s.repository.Delete(ctx, policyID, current.Generation); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return versionConflict(current)
		}
		return err
	}
	return nil
}

// CurrentUsage 查询调用方当前实际执行的 Token 额度
func (s *Service) CurrentUsage(ctx context.Context, callerID string) ([]Usage, error) {
	if _, err := s.callers.Get(ctx, callerID); err != nil {
		return nil, err
	}
	return s.usage.Current(ctx, callerID)
}

func (s *Service) ensureDisplayNameAvailable(ctx context.Context, policyID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(policy resource.TokenQuotaPolicy) (bool, error) {
		if policy.Name != policyID && policy.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("Token 额度策略名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func collectTargetRefs(policies []resource.TokenQuotaPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}

func versionConflict(policy *resource.TokenQuotaPolicy) error {
	return biz.NewVersionConflict(
		policy.Name,
		fmt.Sprintf("Token 额度策略 %q 已被其他用户修改，请刷新后重试", policy.Spec.DisplayName),
	)
}
