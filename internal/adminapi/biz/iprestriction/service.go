// Package iprestriction 处理客户端 IP 访问限制策略的管理规则和资源协作
package iprestriction

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义 IP 访问限制策略管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.IPRestrictionPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.IPRestrictionPolicy, error)
	Create(ctx context.Context, policyID string, spec resource.IPRestrictionPolicySpec) (*resource.IPRestrictionPolicy, error)
	Update(ctx context.Context, policyID string, generation int64, spec resource.IPRestrictionPolicySpec) (*resource.IPRestrictionPolicy, error)
	Delete(ctx context.Context, policyID string, generation int64) error
}

// Service 协调 IPRestrictionPolicy 的目标解析、校验和持久化
type Service struct {
	repository Repository
	targets    *biz.PolicyTargetResolver
}

// PolicyPage 保存一页策略及其目标展示名称
type PolicyPage struct {
	Policies    []resource.IPRestrictionPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单个策略及其目标展示名称
type PolicyView struct {
	Policy      *resource.IPRestrictionPolicy
	TargetNames biz.PolicyTargetNames
}

// NewService 创建 IPRestrictionPolicy 业务服务
func NewService(
	repository Repository,
	gateways biz.GatewayGetter,
	routes biz.RouteGetter,
) *Service {
	return &Service{repository: repository, targets: biz.NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 IPRestrictionPolicy 列表
func (s *Service) List(ctx context.Context, page biz.PageRequest) (PolicyPage, error) {
	result, err := s.repository.ListPage(ctx, page)
	if err != nil {
		return PolicyPage{}, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return PolicyPage{}, err
	}
	return PolicyPage{Policies: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor}, nil
}

// Get 查询单个 IPRestrictionPolicy
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

// Create 创建 IPRestrictionPolicy
func (s *Service) Create(ctx context.Context, spec resource.IPRestrictionPolicySpec) (PolicyView, error) {
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

// Update 使用配置版本乐观更新 IPRestrictionPolicy
func (s *Service) Update(
	ctx context.Context,
	policyID string,
	version int64,
	spec resource.IPRestrictionPolicySpec,
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

// Delete 使用配置版本删除 IPRestrictionPolicy
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

func (s *Service) ensureDisplayNameAvailable(ctx context.Context, policyID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(policy resource.IPRestrictionPolicy) (bool, error) {
		if policy.Name != policyID && policy.Spec.DisplayName == displayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("IP 访问限制策略名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func collectTargetRefs(policies []resource.IPRestrictionPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}

func versionConflict(policy *resource.IPRestrictionPolicy) error {
	return biz.NewVersionConflict(
		policy.Name,
		fmt.Sprintf("IP 访问限制策略 %q 已被其他用户修改，请刷新后重试", policy.Spec.DisplayName),
	)
}
