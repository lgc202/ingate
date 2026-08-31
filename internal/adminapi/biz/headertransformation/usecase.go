// Package headertransformation 处理请求响应 Header 转换策略的业务规则和资源协作。
package headertransformation

import (
	"context"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义请求响应 Header 转换策略管理所需的持久化能力。
type Store interface {
	ListPage(
		ctx context.Context,
		page biz.PageRequest,
	) (biz.PageResult[resource.HeaderTransformationPolicy], error)
	Get(ctx context.Context, policyID string) (*resource.HeaderTransformationPolicy, error)
	Create(
		ctx context.Context,
		policyID string,
		spec resource.HeaderTransformationPolicySpec,
	) (*resource.HeaderTransformationPolicy, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.HeaderTransformationPolicy,
		spec resource.HeaderTransformationPolicySpec,
	) (*resource.HeaderTransformationPolicy, error)
	Delete(ctx context.Context, observed *resource.HeaderTransformationPolicy) error
}

// Usecase 协调请求响应 Header 转换策略的插件、目标校验和持久化。
type Usecase struct {
	policies *biz.PolicyUsecase[
		resource.HeaderTransformationPolicy,
		resource.HeaderTransformationPolicySpec,
	]
	store   Store
	plugins *biz.PluginInstallationChecker
}

// NewUsecase 创建请求响应 Header 转换策略用例。
func NewUsecase(
	store Store,
	routes biz.RouteReader,
	plugins *biz.PluginInstallationChecker,
) *Usecase {
	return &Usecase{
		policies: biz.NewPolicyUsecase(
			store,
			biz.NewRoutePolicyTargetResolver(routes),
			policyAttributes,
			policyTargetRefs,
		),
		store:   store,
		plugins: plugins,
	}
}

// List 返回满足筛选条件的请求响应 Header 转换策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PolicyPage[resource.HeaderTransformationPolicy], error) {
	return uc.policies.List(ctx, page, filter)
}

// Get 返回指定请求响应 Header 转换策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (biz.PolicyView[resource.HeaderTransformationPolicy], error) {
	return uc.policies.Get(ctx, policyID)
}

// Create 创建请求响应 Header 转换策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.HeaderTransformationPolicySpec,
) (biz.PolicyView[resource.HeaderTransformationPolicy], error) {
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return biz.PolicyView[resource.HeaderTransformationPolicy]{}, err
	}
	return uc.policies.Create(ctx, spec)
}

// Replace 使用配置版本完整替换请求响应 Header 转换策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.HeaderTransformationPolicySpec,
) (biz.PolicyView[resource.HeaderTransformationPolicy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return biz.PolicyView[resource.HeaderTransformationPolicy]{}, err
	}

	if current.Generation != expectedGeneration {
		return biz.PolicyView[resource.HeaderTransformationPolicy]{}, biz.ErrResourceVersionConflict
	}
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return biz.PolicyView[resource.HeaderTransformationPolicy]{}, err
	}
	return uc.policies.ReplaceObserved(ctx, current, spec)
}

// Delete 使用配置版本删除请求响应 Header 转换策略。
func (uc *Usecase) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	return uc.policies.Delete(ctx, policyID, expectedGeneration)
}

func (uc *Usecase) checkPluginInstalled(ctx context.Context) error {
	installed, err := uc.plugins.Installed(ctx, resource.WasmPluginPackageTransformer)
	if err != nil {
		return err
	}
	if !installed {
		return errors.Conflict(
			adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
			"请先安装请求响应转换插件",
		)
	}
	return nil
}

func policyAttributes(policy *resource.HeaderTransformationPolicy) biz.PolicyAttributes {
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

func policyTargetRefs(spec resource.HeaderTransformationPolicySpec) []resource.PolicyTargetRef {
	return spec.TargetRefs
}
