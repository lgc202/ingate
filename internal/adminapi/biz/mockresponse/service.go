// Package mockresponse 处理模拟响应策略的管理规则
package mockresponse

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Repository 定义模拟响应策略管理需要的持久化能力
type Repository interface {
	ListPage(ctx context.Context, page biz.PageRequest) (biz.PageResult[resource.MockResponsePolicy], error)
	Get(ctx context.Context, policyID string) (*resource.MockResponsePolicy, error)
	Create(ctx context.Context, policyID string, spec resource.MockResponsePolicySpec) (*resource.MockResponsePolicy, error)
	Update(ctx context.Context, policyID string, generation int64, spec resource.MockResponsePolicySpec) (*resource.MockResponsePolicy, error)
	Delete(ctx context.Context, policyID string, generation int64) error
}

// Service 协调模拟响应策略的目标解析、唯一性和持久化
type Service struct {
	repository Repository
	targets    *biz.PolicyTargetResolver
}

// PolicyPage 保存一页策略及其目标展示名称
type PolicyPage struct {
	Policies    []resource.MockResponsePolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单个策略及其目标展示名称
type PolicyView struct {
	Policy      *resource.MockResponsePolicy
	TargetNames biz.PolicyTargetNames
}

// NewService 创建模拟响应策略业务服务
func NewService(repository Repository, gateways biz.GatewayGetter, routes biz.RouteGetter) *Service {
	return &Service{repository: repository, targets: biz.NewPolicyTargetResolver(gateways, routes)}
}

// List 查询模拟响应策略列表
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

// Get 查询单个模拟响应策略
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

// Create 创建模拟响应策略
func (s *Service) Create(ctx context.Context, spec resource.MockResponsePolicySpec) (PolicyView, error) {
	if err := s.ensureAvailable(ctx, "", spec); err != nil {
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

// Update 使用配置版本乐观更新模拟响应策略
func (s *Service) Update(
	ctx context.Context,
	policyID string,
	version int64,
	spec resource.MockResponsePolicySpec,
) (PolicyView, error) {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}
	if version != current.Generation {
		return PolicyView{}, versionConflict(current)
	}
	if err := s.ensureAvailable(ctx, policyID, spec); err != nil {
		return PolicyView{}, err
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

// Delete 使用配置版本删除模拟响应策略
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

func (s *Service) ensureAvailable(ctx context.Context, policyID string, spec resource.MockResponsePolicySpec) error {
	return biz.VisitPages(ctx, s.repository.ListPage, func(policy resource.MockResponsePolicy) (bool, error) {
		if policy.Name == policyID {
			return false, nil
		}
		if policy.Spec.DisplayName == spec.DisplayName {
			return true, biz.NewRuleViolation(fmt.Sprintf("模拟响应策略名称 %q 已存在", spec.DisplayName))
		}
		if spec.Enabled && policy.Spec.Enabled && targetsOverlap(policy.Spec.TargetRefs, spec.TargetRefs) {
			return true, biz.NewRuleViolation(fmt.Sprintf("目标路由已应用模拟响应策略 %q，请先调整其生效范围", policy.Spec.DisplayName))
		}
		return false, nil
	})
}

func targetsOverlap(left, right []resource.PolicyTargetRef) bool {
	for _, target := range left {
		if slices.Contains(right, target) {
			return true
		}
	}
	return false
}

func collectTargetRefs(policies []resource.MockResponsePolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}

func versionConflict(policy *resource.MockResponsePolicy) error {
	return biz.NewVersionConflict(
		policy.Name,
		fmt.Sprintf("模拟响应策略 %q 已被其他用户修改，请刷新后重试", policy.Spec.DisplayName),
	)
}
