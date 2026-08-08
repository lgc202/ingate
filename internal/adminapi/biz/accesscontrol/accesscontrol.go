// Package accesscontrol 实现访问控制策略管理用例
package accesscontrol

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义访问控制策略用例需要的持久化能力
type Repository interface {
	List(context.Context) ([]resource.AccessControlPolicy, error)
	Get(context.Context, string) (*resource.AccessControlPolicy, error)
	Create(context.Context, string, resource.AccessControlPolicySpec) error
	Update(context.Context, string, int64, resource.AccessControlPolicySpec) error
	Delete(context.Context, string) error
}

// Usecase 承载 AccessControlPolicy 管理用例
type Usecase struct {
	repository Repository
	targets    *biz.PolicyTargetResolver
}

// ListResult 保存策略列表及其目标展示名称
type ListResult struct {
	Policies    []resource.AccessControlPolicy
	TargetNames biz.PolicyTargetNames
}

// Result 保存单个策略及其目标展示名称
type Result struct {
	Policy      *resource.AccessControlPolicy
	TargetNames biz.PolicyTargetNames
}

// NewUsecase 创建访问控制策略用例
func NewUsecase(
	repository Repository,
	gateways biz.GatewayLister,
	routes biz.RouteLister,
) *Usecase {
	return &Usecase{repository: repository, targets: biz.NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 AccessControlPolicy 列表
func (u *Usecase) List(ctx context.Context) (*ListResult, error) {
	policies, err := u.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	targetNames, err := u.targets.DisplayNames(ctx, accessControlPolicyTargetRefs(policies))
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: policies, TargetNames: targetNames}, nil
}

// Get 查询单个 AccessControlPolicy
func (u *Usecase) Get(ctx context.Context, policyID string) (*Result, error) {
	policy, err := u.repository.Get(ctx, policyID)
	if err != nil {
		return nil, err
	}
	targetNames, err := u.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return nil, err
	}
	return &Result{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建 AccessControlPolicy
func (u *Usecase) Create(ctx context.Context, spec resource.AccessControlPolicySpec) (string, error) {
	if err := u.validateNameUnique(ctx, spec.DisplayName, ""); err != nil {
		return "", err
	}
	if err := u.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := u.repository.Create(ctx, id, spec); err != nil {
		return "", err
	}
	return id, nil
}

// Update 更新 AccessControlPolicy
func (u *Usecase) Update(ctx context.Context, policyID, version string, spec resource.AccessControlPolicySpec) error {
	current, err := u.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := u.validateNameUnique(ctx, spec.DisplayName, policyID); err != nil {
		return err
	}
	if err := u.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return err
	}
	if err := u.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// SetEnabled 设置 AccessControlPolicy 启用状态
func (u *Usecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	current, err := u.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := u.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("访问控制策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 AccessControlPolicy
func (u *Usecase) Delete(ctx context.Context, policyID string) error {
	return u.repository.Delete(ctx, policyID)
}

func (u *Usecase) validateNameUnique(ctx context.Context, name, excludeID string) error {
	policies, err := u.repository.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range policies {
		if current.Name == excludeID {
			continue
		}
		if current.Spec.DisplayName == name {
			return biz.NewUserError(fmt.Sprintf("访问控制策略名称 %q 已存在", name))
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
