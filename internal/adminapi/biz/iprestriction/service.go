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
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.IPRestrictionPolicy], error)
	Get(context.Context, string) (*resource.IPRestrictionPolicy, error)
	Create(context.Context, string, resource.IPRestrictionPolicySpec) (*resource.IPRestrictionPolicy, error)
	Update(context.Context, string, int64, resource.IPRestrictionPolicySpec) (*resource.IPRestrictionPolicy, error)
	Delete(context.Context, string, int64) error
}

// Service 协调 IPRestrictionPolicy 的目标解析、校验和持久化
type Service struct {
	repository Repository
	targets    *biz.PolicyTargetResolver
}

// ListResult 保存策略列表及其目标展示名称
type ListResult struct {
	Policies    []resource.IPRestrictionPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// Result 保存单个策略及其目标展示名称
type Result struct {
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
func (s *Service) List(ctx context.Context, page biz.PageRequest) (*ListResult, error) {
	result, err := s.repository.ListPage(ctx, page)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, targetRefs(result.Items))
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor}, nil
}

// Get 查询单个 IPRestrictionPolicy
func (s *Service) Get(ctx context.Context, policyID string) (*Result, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &Result{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 IPRestrictionPolicy
func (s *Service) Create(ctx context.Context, spec resource.IPRestrictionPolicySpec) (*Result, error) {
	if err := s.validateDisplayName(ctx, "", spec.DisplayName); err != nil {
		return nil, err
	}
	targetNames, err := s.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	policy, err := s.repository.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return nil, err
	}
	return &Result{Policy: policy, TargetNames: targetNames}, nil
}

// Update 使用配置版本乐观更新 IPRestrictionPolicy
func (s *Service) Update(
	ctx context.Context,
	policyID string,
	version int64,
	spec resource.IPRestrictionPolicySpec,
) (*Result, error) {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	if version != current.Generation {
		return nil, versionConflict(current)
	}
	if spec.DisplayName != current.Spec.DisplayName {
		if err := s.validateDisplayName(ctx, policyID, spec.DisplayName); err != nil {
			return nil, err
		}
	}
	targetNames, err := s.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	updated, err := s.repository.Update(ctx, policyID, current.Generation, spec)
	if err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return nil, versionConflict(current)
		}
		return nil, err
	}
	return &Result{Policy: updated, TargetNames: targetNames}, nil
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

func (s *Service) validateDisplayName(ctx context.Context, policyID, displayName string) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(policy resource.IPRestrictionPolicy) (bool, error) {
		if policy.Name != policyID && policy.Spec.DisplayName == displayName {
			return true, biz.NewUserError(fmt.Sprintf("IP 访问限制策略名称 %q 已存在", displayName))
		}
		return false, nil
	})
}

func targetRefs(policies []resource.IPRestrictionPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}

func versionConflict(policy *resource.IPRestrictionPolicy) error {
	return biz.NewVersionConflictError(
		policy.Name,
		fmt.Sprintf("IP 访问限制策略 %q 已被其他用户修改，请刷新后重试", policy.Spec.DisplayName),
	)
}
