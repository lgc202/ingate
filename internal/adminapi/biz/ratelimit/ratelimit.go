// Package ratelimit 实现请求限流策略管理用例
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

// Repository 定义限流策略用例需要的持久化能力
type Repository interface {
	ListPage(context.Context, biz.PageRequest) (biz.PageResult[resource.RateLimitPolicy], error)
	Get(context.Context, string) (*resource.RateLimitPolicy, error)
	Create(context.Context, string, resource.RateLimitPolicySpec) error
	Update(context.Context, string, int64, resource.RateLimitPolicySpec) error
	Delete(context.Context, string) error
}

// Usecase 承载 RateLimitPolicy 管理用例
type Usecase struct {
	repository Repository
	targets    *biz.PolicyTargetResolver
}

// ListResult 保存策略列表及其目标展示名称
type ListResult struct {
	Policies    []resource.RateLimitPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// Result 保存单个策略及其目标展示名称
type Result struct {
	Policy      *resource.RateLimitPolicy
	TargetNames biz.PolicyTargetNames
}

// NewUsecase 创建请求限流策略用例
func NewUsecase(
	repository Repository,
	gateways biz.GatewayGetter,
	routes biz.RouteGetter,
) *Usecase {
	return &Usecase{repository: repository, targets: biz.NewPolicyTargetResolver(gateways, routes)}
}

// List 查询 RateLimitPolicy 列表
func (u *Usecase) List(ctx context.Context, page biz.PageRequest) (*ListResult, error) {
	result, err := u.repository.ListPage(ctx, page)
	if err != nil {
		return nil, err
	}
	targetNames, err := u.targets.DisplayNames(ctx, rateLimitPolicyTargetRefs(result.Items))
	if err != nil {
		return nil, err
	}
	return &ListResult{Policies: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor}, nil
}

// Get 查询单个 RateLimitPolicy
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

// Create 创建 RateLimitPolicy
func (u *Usecase) Create(ctx context.Context, spec resource.RateLimitPolicySpec) (string, error) {
	if err := u.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return "", err
	}
	id := uuid.NewString()
	if err := u.repository.Create(ctx, id, spec); err != nil {
		return "", biz.DisplayNameConflict(err, "限流策略", spec.DisplayName)
	}
	return id, nil
}

// Update 更新 RateLimitPolicy
func (u *Usecase) Update(ctx context.Context, policyID, version string, spec resource.RateLimitPolicySpec) error {
	current, err := u.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if version != strconv.FormatInt(current.Generation, 10) {
		return biz.NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
	}
	if err := u.targets.Validate(ctx, spec.TargetRefs); err != nil {
		return err
	}
	if err := u.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return biz.DisplayNameConflict(err, "限流策略", spec.DisplayName)
	}
	return nil
}

// SetEnabled 设置 RateLimitPolicy 启用状态
func (u *Usecase) SetEnabled(ctx context.Context, policyID string, enabled bool) error {
	current, err := u.repository.Get(ctx, policyID)
	if err != nil {
		return err
	}
	spec := current.Spec
	spec.Enabled = enabled
	if err := u.repository.Update(ctx, policyID, current.Generation, spec); err != nil {
		if errors.Is(err, biz.ErrResourceVersionConflict) {
			return biz.NewUserError(fmt.Sprintf("限流策略 %q 已被更新，请刷新后重试", current.Spec.DisplayName))
		}
		return err
	}
	return nil
}

// Delete 删除 RateLimitPolicy
func (u *Usecase) Delete(ctx context.Context, policyID string) error {
	return u.repository.Delete(ctx, policyID)
}

func rateLimitPolicyTargetRefs(policies []resource.RateLimitPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for _, policy := range policies {
		refs = append(refs, policy.Spec.TargetRefs...)
	}
	return refs
}
