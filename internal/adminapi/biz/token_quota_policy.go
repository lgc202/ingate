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

const policyNotFoundMessage = "Token 配额策略不存在"

// TokenQuotaPolicyRepository 定义 Token 配额策略用例需要的持久化能力
type TokenQuotaPolicyRepository interface {
	List(context.Context) ([]resource.TokenQuotaPolicy, error)
	Get(context.Context, string) (*resource.TokenQuotaPolicy, error)
	Create(context.Context, string, resource.TokenQuotaPolicySpec) error
	Update(context.Context, string, int64, resource.TokenQuotaPolicySpec) error
	Delete(context.Context, string) error
}

// TokenQuotaPolicyUsecase 承载 TokenQuotaPolicy 管理用例
type TokenQuotaPolicyUsecase struct {
	repository TokenQuotaPolicyRepository
	targets    *PolicyTargetResolver
	writeMu    sync.Mutex
}

// TokenQuotaPolicyList 保存策略列表及其目标展示名称
type TokenQuotaPolicyList struct {
	Policies    []resource.TokenQuotaPolicy
	TargetNames PolicyTargetNames
}

// TokenQuotaPolicyResult 保存单个策略及其目标展示名称
type TokenQuotaPolicyResult struct {
	Policy      *resource.TokenQuotaPolicy
	TargetNames PolicyTargetNames
}

// NewTokenQuotaPolicyUsecase 创建 Token 配额策略用例
func NewTokenQuotaPolicyUsecase(
	repository TokenQuotaPolicyRepository,
	gateways GatewayRepository,
	routes RouteRepository,
) *TokenQuotaPolicyUsecase {
	return &TokenQuotaPolicyUsecase{repository: repository, targets: NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 TokenQuotaPolicy 列表
func (s *TokenQuotaPolicyUsecase) List(ctx context.Context) (*TokenQuotaPolicyList, error) {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, tokenQuotaPolicyTargetRefs(policies))
	if err != nil {
		return nil, err
	}
	return &TokenQuotaPolicyList{Policies: policies, TargetNames: targetNames}, nil
}

// Get 查询单个 TokenQuotaPolicy
func (s *TokenQuotaPolicyUsecase) Get(ctx context.Context, policyID string) (*TokenQuotaPolicyResult, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			return nil, NewUserError(policyNotFoundMessage)
		}
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &TokenQuotaPolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 TokenQuotaPolicy
func (s *TokenQuotaPolicyUsecase) Create(ctx context.Context, spec resource.TokenQuotaPolicySpec) (string, error) {
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

// Update 更新 TokenQuotaPolicy
func (s *TokenQuotaPolicyUsecase) Update(ctx context.Context, policyID, version string, spec resource.TokenQuotaPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			return NewUserError(policyNotFoundMessage)
		}
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return NewUserError(fmt.Sprintf("Token 配额策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, policyID); err != nil {
		return err
	}
	if err := s.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return err
	}
	if err := s.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		switch {
		case errors.Is(err, ErrResourceNotFound):
			return NewUserError(policyNotFoundMessage)
		case errors.Is(err, ErrResourceVersionConflict):
			return NewUserError(fmt.Sprintf("Token 配额策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		default:
			return err
		}
	}
	return nil
}

// SetEnabled 设置 TokenQuotaPolicy 启用状态
func (s *TokenQuotaPolicyUsecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		if errors.Is(err, ErrResourceNotFound) {
			return NewUserError(policyNotFoundMessage)
		}
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := s.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		switch {
		case errors.Is(err, ErrResourceNotFound):
			return NewUserError(policyNotFoundMessage)
		case errors.Is(err, ErrResourceVersionConflict):
			return NewUserError(fmt.Sprintf("Token 配额策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		default:
			return err
		}
	}
	return nil
}

// Delete 删除 TokenQuotaPolicy
func (s *TokenQuotaPolicyUsecase) Delete(ctx context.Context, policyID string) error {
	err := s.repository.Delete(ctx, policyID)
	if errors.Is(err, ErrResourceNotFound) {
		return NewUserError(policyNotFoundMessage)
	}
	return err
}

func (s *TokenQuotaPolicyUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("Token 配额策略名称 %q 已存在", name))
		}
	}
	return nil
}

func tokenQuotaPolicyTargetRefs(policies []resource.TokenQuotaPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
