// Package headertransformation 处理请求响应 Header 转换策略的业务规则和资源协作。
package headertransformation

import (
	"context"

	"github.com/google/uuid"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义请求响应 Header 转换策略管理所需的持久化能力。
type Store interface {
	ListPage(
		ctx context.Context,
		page pagination.Request,
	) (pagination.Result[resource.HeaderTransformationPolicy], error)
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
	store   Store
	targets *policy.TargetResolver
	plugins *plugin.InstallationChecker
}

// NewUsecase 创建请求响应 Header 转换策略用例。
func NewUsecase(
	store Store,
	routes policy.RouteReader,
	plugins *plugin.InstallationChecker,
) *Usecase {
	return &Usecase{
		store:   store,
		targets: policy.NewRouteTargetResolver(routes),
		plugins: plugins,
	}
}

// List 返回满足筛选条件的请求响应 Header 转换策略。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter resourceview.Filter,
) (policy.Page[resource.HeaderTransformationPolicy], error) {
	result, err := resourceview.FilterPage(ctx, page, uc.store.ListPage, func(item resource.HeaderTransformationPolicy) bool {
		return filter.Match(item.Spec.DisplayName, item.Spec.Enabled, policyStatus(&item))
	})
	if err != nil {
		return policy.Page[resource.HeaderTransformationPolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return policy.Page[resource.HeaderTransformationPolicy]{}, err
	}
	return policy.Page[resource.HeaderTransformationPolicy]{
		Items: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor,
	}, nil
}

// Get 返回指定请求响应 Header 转换策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (policy.View[resource.HeaderTransformationPolicy], error) {
	item, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, item.Spec.TargetRefs)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	return policy.View[resource.HeaderTransformationPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Create 创建请求响应 Header 转换策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.HeaderTransformationPolicySpec,
) (policy.View[resource.HeaderTransformationPolicy], error) {
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	item, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	return policy.View[resource.HeaderTransformationPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换请求响应 Header 转换策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.HeaderTransformationPolicySpec,
) (policy.View[resource.HeaderTransformationPolicy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}

	if current.Generation != expectedGeneration {
		return policy.View[resource.HeaderTransformationPolicy]{}, apperror.ResourceVersionConflict()
	}
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	item, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return policy.View[resource.HeaderTransformationPolicy]{}, err
	}
	return policy.View[resource.HeaderTransformationPolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Delete 使用配置版本删除请求响应 Header 转换策略。
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

func (uc *Usecase) checkPluginInstalled(ctx context.Context) error {
	installed, err := uc.plugins.Installed(ctx, resource.WasmPluginPackageTransformer)
	if err != nil {
		return err
	}
	if !installed {
		return adminv1.ErrorBusinessRuleViolation("请先安装请求响应转换插件")
	}
	return nil
}

func policyStatus(item *resource.HeaderTransformationPolicy) resourceview.Status {
	return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
}

func collectTargetRefs(items []resource.HeaderTransformationPolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for i := range items {
		refs = append(refs, items[i].Spec.TargetRefs...)
	}
	return refs
}
