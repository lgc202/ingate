// Package iprestriction 处理客户端 IP 访问限制策略的业务规则和资源协作。
package iprestriction

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义客户端 IP 访问限制策略管理所需的持久化能力。
type Store interface {
	ListPage(
		ctx context.Context,
		page biz.PageRequest,
	) (biz.PageResult[resource.IPRestrictionPolicy], error)
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
	policies *biz.PolicyUsecase[resource.IPRestrictionPolicy, resource.IPRestrictionPolicySpec]
}

// NewUsecase 创建客户端 IP 访问限制策略用例。
func NewUsecase(
	store Store,
	gateways biz.GatewayReader,
	routes biz.RouteReader,
) *Usecase {
	return &Usecase{
		policies: biz.NewPolicyUsecase(
			store,
			biz.NewPolicyTargetResolver(gateways, routes),
			policyAttributes,
			policyTargetRefs,
		),
	}
}

// List 返回满足筛选条件的客户端 IP 访问限制策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PolicyPage[resource.IPRestrictionPolicy], error) {
	return uc.policies.List(ctx, page, filter)
}

// Get 返回指定客户端 IP 访问限制策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (biz.PolicyView[resource.IPRestrictionPolicy], error) {
	return uc.policies.Get(ctx, policyID)
}

// Create 创建客户端 IP 访问限制策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.IPRestrictionPolicySpec,
) (biz.PolicyView[resource.IPRestrictionPolicy], error) {
	return uc.policies.Create(ctx, spec)
}

// Replace 使用配置版本完整替换客户端 IP 访问限制策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.IPRestrictionPolicySpec,
) (biz.PolicyView[resource.IPRestrictionPolicy], error) {
	return uc.policies.Replace(ctx, policyID, expectedGeneration, spec)
}

// Delete 使用配置版本删除客户端 IP 访问限制策略。
func (uc *Usecase) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	return uc.policies.Delete(ctx, policyID, expectedGeneration)
}

func policyAttributes(policy *resource.IPRestrictionPolicy) biz.PolicyAttributes {
	return biz.PolicyAttributes{
		Generation:  policy.Generation,
		DisplayName: policy.Spec.DisplayName,
		Enabled:     policy.Spec.Enabled,
		TargetRefs:  policy.Spec.TargetRefs,
		Status: biz.PolicyStatus(
			policy.Generation,
			policy.Spec.Enabled,
			len(policy.Spec.TargetRefs),
			policy.Status.Conditions,
		),
	}
}

func policyTargetRefs(spec resource.IPRestrictionPolicySpec) []resource.PolicyTargetRef {
	return spec.TargetRefs
}
