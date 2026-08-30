// Package headertransformation 处理请求响应 Header 转换策略的业务规则和资源协作。
package headertransformation

import (
	"context"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"

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

// PolicyPage 保存一页请求响应 Header 转换策略及其目标展示名称。
type PolicyPage struct {
	Items       []resource.HeaderTransformationPolicy
	TargetNames biz.PolicyTargetNames
	NextCursor  string
}

// PolicyView 保存单条请求响应 Header 转换策略及其目标展示名称。
type PolicyView struct {
	Policy      *resource.HeaderTransformationPolicy
	TargetNames biz.PolicyTargetNames
}

// Usecase 协调请求响应 Header 转换策略的插件、目标校验和持久化。
type Usecase struct {
	store   Store
	targets *biz.PolicyTargetResolver
	plugins *biz.PluginInstallationChecker
}

// NewUsecase 创建请求响应 Header 转换策略用例。
func NewUsecase(
	store Store,
	routes biz.RouteLister,
	plugins *biz.PluginInstallationChecker,
) *Usecase {
	return &Usecase{
		store:   store,
		targets: biz.NewRoutePolicyTargetResolver(routes),
		plugins: plugins,
	}
}

// List 返回满足筛选条件的请求响应 Header 转换策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (PolicyPage, error) {
	result, err := biz.FilterPage(
		ctx,
		page,
		uc.store.ListPage,
		func(policy resource.HeaderTransformationPolicy) bool {
			status := biz.PolicyStatus(
				policy.Generation,
				policy.Spec.Enabled,
				len(policy.Spec.TargetRefs),
				policy.Status.Conditions,
			)
			return filter.Match(policy.Spec.DisplayName, policy.Spec.Enabled, status)
		},
	)
	if err != nil {
		return PolicyPage{}, err
	}

	targetNames, err := uc.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return PolicyPage{}, err
	}
	return PolicyPage{
		Items:       result.Items,
		TargetNames: targetNames,
		NextCursor:  result.NextCursor,
	}, nil
}

// Get 返回指定请求响应 Header 转换策略。
func (uc *Usecase) Get(ctx context.Context, policyID string) (PolicyView, error) {
	policy, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, policy.Spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Create 创建请求响应 Header 转换策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.HeaderTransformationPolicySpec,
) (PolicyView, error) {
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return PolicyView{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}

	policyID := uuid.NewString()
	policy, err := uc.store.Create(ctx, policyID, spec)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换请求响应 Header 转换策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.HeaderTransformationPolicySpec,
) (PolicyView, error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return PolicyView{}, err
	}

	if current.Generation != expectedGeneration {
		return PolicyView{}, biz.ErrResourceVersionConflict
	}
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return PolicyView{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return PolicyView{}, err
	}

	policy, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return PolicyView{}, err
	}
	return PolicyView{Policy: policy, TargetNames: targetNames}, nil
}

// Delete 删除请求响应 Header 转换策略。
func (uc *Usecase) Delete(ctx context.Context, policyID string, expectedGeneration int64) error {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return err
	}
	if current.Generation != expectedGeneration {
		return biz.ErrResourceVersionConflict
	}
	return uc.store.Delete(ctx, current)
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

func collectTargetRefs(policies []resource.HeaderTransformationPolicy) []resource.PolicyTargetRef {
	targetCount := 0
	for i := range policies {
		targetCount += len(policies[i].Spec.TargetRefs)
	}

	refs := make([]resource.PolicyTargetRef, 0, targetCount)
	for i := range policies {
		refs = append(refs, policies[i].Spec.TargetRefs...)
	}
	return refs
}
