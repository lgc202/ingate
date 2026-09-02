// Package iprestriction 处理客户端 IP 访问限制策略的业务规则和资源协作。
package iprestriction

import (
	"context"

	"github.com/google/uuid"

	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义客户端 IP 访问限制策略管理所需的持久化能力。
type Store interface {
	ListPage(
		ctx context.Context,
		page pagination.Request,
	) (pagination.Result[resource.IPRestrictionPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.IPRestrictionPolicy, error)
	Create(
		ctx context.Context,
		policyID string,
		spec resource.IPRestrictionPolicySpec,
	) (*resource.IPRestrictionPolicy, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.IPRestrictionPolicy,
		spec resource.IPRestrictionPolicySpec,
	) (*resource.IPRestrictionPolicy, error)
	Delete(ctx context.Context, observed *resource.IPRestrictionPolicy) error
}

// Usecase 提供客户端 IP 访问限制策略管理能力。
type Usecase struct {
	store   Store
	targets *policy.PolicyTargetResolver
}

// NewUsecase 创建客户端 IP 访问限制策略用例。
func NewUsecase(
	store Store,
	gateways policy.GatewayReader,
	routes policy.RouteReader,
) *Usecase {
	return &Usecase{
		store:   store,
		targets: policy.NewPolicyTargetResolver(gateways, routes),
	}
}

// List 返回满足筛选条件的客户端 IP 访问限制策略。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter resourceview.Filter,
) (policy.Page[resource.IPRestrictionPolicy], error) {
	result, err := resourceview.FilterPage(ctx, page, uc.store.ListPage, func(item resource.IPRestrictionPolicy) bool {
		return filter.Match(item.Spec.DisplayName, item.Spec.Enabled, policyStatus(&item))
	})
	if err != nil {
		return policy.Page[resource.IPRestrictionPolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return policy.Page[resource.IPRestrictionPolicy]{}, err
	}
	return policy.Page[resource.IPRestrictionPolicy]{
		Items: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor,
	}, nil
}

// Get 返回指定客户端 IP 访问限制策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (policy.View[resource.IPRestrictionPolicy], error) {
	item, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, item.Spec.TargetRefs)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	return policy.View[resource.IPRestrictionPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Create 创建客户端 IP 访问限制策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.IPRestrictionPolicySpec,
) (policy.View[resource.IPRestrictionPolicy], error) {
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	item, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	return policy.View[resource.IPRestrictionPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换客户端 IP 访问限制策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.IPRestrictionPolicySpec,
) (policy.View[resource.IPRestrictionPolicy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	if current.Generation != expectedGeneration {
		return policy.View[resource.IPRestrictionPolicy]{}, apperror.ResourceVersionConflict()
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	item, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return policy.View[resource.IPRestrictionPolicy]{}, err
	}
	return policy.View[resource.IPRestrictionPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Delete 使用配置版本删除客户端 IP 访问限制策略。
func (uc *Usecase) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if current.Generation != expectedGeneration {
		return apperror.ResourceVersionConflict()
	}
	return uc.store.Delete(ctx, current)
}

func policyStatus(item *resource.IPRestrictionPolicy) resourceview.Status {
	return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
}

func collectTargetRefs(items []resource.IPRestrictionPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for i := range items {
		refs = append(refs, items[i].Spec.TargetRefs...)
	}
	return refs
}
