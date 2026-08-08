package biz

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"

	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// RateLimitPolicyRepository 定义限流策略用例需要的持久化能力
type RateLimitPolicyRepository interface {
	List(context.Context) ([]resource.RateLimitPolicy, error)
	Get(context.Context, string) (*resource.RateLimitPolicy, error)
	Create(context.Context, string, resource.RateLimitPolicySpec) error
	Update(context.Context, string, int64, resource.RateLimitPolicySpec) error
	Delete(context.Context, string) error
}

// RateLimitPolicyUsecase 承载 RateLimitPolicy 管理用例
type RateLimitPolicyUsecase struct {
	repository RateLimitPolicyRepository
	targets    *PolicyTargetResolver
	writeMu    sync.Mutex
}

// RateLimitPolicyList 保存策略列表及其目标展示名称
type RateLimitPolicyList struct {
	Policies    []resource.RateLimitPolicy
	TargetNames PolicyTargetNames
}

// RateLimitPolicyResult 保存单个策略及其目标展示名称
type RateLimitPolicyResult struct {
	Policy      *resource.RateLimitPolicy
	TargetNames PolicyTargetNames
}

// NewRateLimitPolicyUsecase 创建请求限流策略用例
func NewRateLimitPolicyUsecase(
	repository RateLimitPolicyRepository,
	gateways GatewayRepository,
	routes RouteRepository,
) *RateLimitPolicyUsecase {
	return &RateLimitPolicyUsecase{repository: repository, targets: NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 RateLimitPolicy 列表
func (s *RateLimitPolicyUsecase) List(ctx context.Context) (*RateLimitPolicyList, error) {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, rateLimitPolicyTargetRefs(policies))
	if err != nil {
		return nil, err
	}
	return &RateLimitPolicyList{Policies: policies, TargetNames: targetNames}, nil
}

// Get 查询单个 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Get(ctx context.Context, policyID string) (*RateLimitPolicyResult, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &RateLimitPolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Create(ctx context.Context, spec resource.RateLimitPolicySpec) (string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := s.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := s.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Update(ctx context.Context, policyID, version string, spec resource.RateLimitPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, policyID); err != nil {
		return err
	}
	if err := s.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return err
	}
	if err := s.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, ErrResourceVersionConflict) {
			return NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 设置 RateLimitPolicy 启用状态
func (s *RateLimitPolicyUsecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := s.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, ErrResourceVersionConflict) {
			return NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 RateLimitPolicy
func (s *RateLimitPolicyUsecase) Delete(ctx context.Context, policyID string) error {
	return s.repository.Delete(ctx, policyID)
}

func (s *RateLimitPolicyUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("限流策略名称 %q 已存在", name))
		}
	}
	return nil
}

func rateLimitPolicyTargetRefs(policies []resource.RateLimitPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
