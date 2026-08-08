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

// AccessControlPolicyRepository 定义访问控制策略用例需要的持久化能力
type AccessControlPolicyRepository interface {
	List(context.Context) ([]resource.AccessControlPolicy, error)
	Get(context.Context, string) (*resource.AccessControlPolicy, error)
	Create(context.Context, string, resource.AccessControlPolicySpec) error
	Update(context.Context, string, int64, resource.AccessControlPolicySpec) error
	Delete(context.Context, string) error
}

// AccessControlPolicyUsecase 承载 AccessControlPolicy 管理用例
type AccessControlPolicyUsecase struct {
	repository AccessControlPolicyRepository
	targets    *PolicyTargetResolver
	writeMu    sync.Mutex
}

// AccessControlPolicyList 保存策略列表及其目标展示名称
type AccessControlPolicyList struct {
	Policies    []resource.AccessControlPolicy
	TargetNames PolicyTargetNames
}

// AccessControlPolicyResult 保存单个策略及其目标展示名称
type AccessControlPolicyResult struct {
	Policy      *resource.AccessControlPolicy
	TargetNames PolicyTargetNames
}

// NewAccessControlPolicyUsecase 创建访问控制策略用例
func NewAccessControlPolicyUsecase(
	repository AccessControlPolicyRepository,
	gateways GatewayRepository,
	routes RouteRepository,
) *AccessControlPolicyUsecase {
	return &AccessControlPolicyUsecase{repository: repository, targets: NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 AccessControlPolicy 列表
func (s *AccessControlPolicyUsecase) List(ctx context.Context) (*AccessControlPolicyList, error) {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, accessControlPolicyTargetRefs(policies))
	if err != nil {
		return nil, err
	}
	return &AccessControlPolicyList{Policies: policies, TargetNames: targetNames}, nil
}

// Get 查询单个 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Get(ctx context.Context, policyID string) (*AccessControlPolicyResult, error) {
	policy, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := s.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &AccessControlPolicyResult{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Create(ctx context.Context, spec resource.AccessControlPolicySpec) (string, error) {
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

// Update 更新 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Update(ctx context.Context, policyID, version string, spec resource.AccessControlPolicySpec) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := s.validateNameUnique(ctx, spec.DisplayName, policyID); err != nil {
		return err
	}
	if err := s.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return err
	}
	if err := s.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, ErrResourceVersionConflict) {
			return NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 设置 AccessControlPolicy 启用状态
func (s *AccessControlPolicyUsecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	current, err := s.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := s.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, ErrResourceVersionConflict) {
			return NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 AccessControlPolicy
func (s *AccessControlPolicyUsecase) Delete(ctx context.Context, policyID string) error {
	return s.repository.Delete(ctx, policyID)
}

func (s *AccessControlPolicyUsecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := s.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return NewUserError(fmt.Sprintf("访问控制策略名称 %q 已存在", name))
		}
	}
	return nil
}

func accessControlPolicyTargetRefs(policies []resource.AccessControlPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
