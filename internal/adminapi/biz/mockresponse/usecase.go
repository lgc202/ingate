// Package mockresponse 处理模拟响应策略的业务规则和资源协作。
package mockresponse

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/apperror"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resourceview"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义模拟响应策略管理所需的持久化能力。
type Store interface {
	ListPage(
		ctx context.Context,
		page pagination.Request,
	) (pagination.Result[resource.MockResponsePolicy], error)
	Get(ctx context.Context, policyID string) (*resource.MockResponsePolicy, error)
	Create(
		ctx context.Context,
		policyID string,
		spec resource.MockResponsePolicySpec,
	) (*resource.MockResponsePolicy, error)
	ReplaceSpec(
		ctx context.Context,
		observed *resource.MockResponsePolicy,
		spec resource.MockResponsePolicySpec,
	) (*resource.MockResponsePolicy, error)
	Delete(ctx context.Context, observed *resource.MockResponsePolicy) error
}

// Usecase 协调模拟响应策略的插件、目标、独占范围校验和持久化。
type Usecase struct {
	store   Store
	targets *policy.PolicyTargetResolver
	plugins *plugin.PluginInstallationChecker
}

// NewUsecase 创建模拟响应策略用例。
func NewUsecase(
	store Store,
	routes policy.RouteReader,
	plugins *plugin.PluginInstallationChecker,
) *Usecase {
	return &Usecase{
		store:   store,
		targets: policy.NewRoutePolicyTargetResolver(routes),
		plugins: plugins,
	}
}

// List 返回满足筛选条件的模拟响应策略。
func (uc *Usecase) List(
	ctx context.Context,
	page pagination.Request,
	filter resourceview.Filter,
) (policy.Page[resource.MockResponsePolicy], error) {
	result, err := resourceview.FilterPage(ctx, page, uc.store.ListPage, func(item resource.MockResponsePolicy) bool {
		return filter.Match(item.Spec.DisplayName, item.Spec.Enabled, policyStatus(&item))
	})
	if err != nil {
		return policy.Page[resource.MockResponsePolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, collectTargetRefs(result.Items))
	if err != nil {
		return policy.Page[resource.MockResponsePolicy]{}, err
	}
	return policy.Page[resource.MockResponsePolicy]{
		Items: result.Items, TargetNames: targetNames, NextCursor: result.NextCursor,
	}, nil
}

// Get 返回指定模拟响应策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (policy.View[resource.MockResponsePolicy], error) {
	item, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	targetNames, err := uc.targets.DisplayNames(ctx, item.Spec.TargetRefs)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	return policy.View[resource.MockResponsePolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Create 创建模拟响应策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.MockResponsePolicySpec,
) (policy.View[resource.MockResponsePolicy], error) {
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	if err := uc.checkTargetClaimsAvailable(ctx, "", spec); err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	item, err := uc.store.Create(ctx, uuid.NewString(), spec)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	return policy.View[resource.MockResponsePolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Replace 使用配置版本完整替换模拟响应策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.MockResponsePolicySpec,
) (policy.View[resource.MockResponsePolicy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}

	if current.Generation != expectedGeneration {
		return policy.View[resource.MockResponsePolicy]{}, apperror.ResourceVersionConflict()
	}
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	if err := uc.checkTargetClaimsAvailable(ctx, policyID, spec); err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	targetNames, err := uc.targets.Resolve(ctx, spec.TargetRefs)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	item, err := uc.store.ReplaceSpec(ctx, current, spec)
	if err != nil {
		return policy.View[resource.MockResponsePolicy]{}, err
	}
	return policy.View[resource.MockResponsePolicy]{Policy: item, TargetNames: targetNames}, nil
}

// Delete 使用配置版本删除模拟响应策略。
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
	installed, err := uc.plugins.Installed(ctx, resource.WasmPluginPackageMockResponse)
	if err != nil {
		return err
	}
	if !installed {
		return adminv1.ErrorBusinessRuleViolation("请先安装模拟响应插件")
	}
	return nil
}

// checkTargetClaimsAvailable 预检启用策略的目标占用，以便立即返回明确错误。
// 并发写入产生的冲突仍由 Controller status 最终裁决。
func (uc *Usecase) checkTargetClaimsAvailable(
	ctx context.Context,
	excludedPolicyID string,
	spec resource.MockResponsePolicySpec,
) error {
	desiredTargets := make(map[resource.PolicyTargetRef]bool, len(spec.TargetRefs))
	if spec.Enabled {
		for _, target := range spec.TargetRefs {
			desiredTargets[target] = true
		}
	}

	return pagination.VisitPages(
		ctx,
		uc.store.ListPage,
		func(candidate resource.MockResponsePolicy) (bool, error) {
			if candidate.Name == excludedPolicyID {
				return false, nil
			}
			if !candidate.Spec.Enabled {
				return false, nil
			}
			for _, target := range candidate.Spec.TargetRefs {
				if _, overlaps := desiredTargets[target]; overlaps {
					return false, adminv1.ErrorResourceConflict("%s", fmt.Sprintf(
						"目标路由已应用模拟响应策略 %q，请先调整其生效范围",
						candidate.Spec.DisplayName,
					),
					)
				}
			}
			return false, nil
		},
	)
}

func policyStatus(item *resource.MockResponsePolicy) resourceview.Status {
	return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
}

func collectTargetRefs(items []resource.MockResponsePolicy) []resource.PolicyTargetRef {
	var refs []resource.PolicyTargetRef
	for i := range items {
		refs = append(refs, items[i].Spec.TargetRefs...)
	}
	return refs
}
