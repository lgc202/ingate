// Package mockresponse 处理模拟响应策略的业务规则和资源协作。
package mockresponse

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义模拟响应策略管理所需的持久化能力。
type Store interface {
	ListPage(
		ctx context.Context,
		page biz.PageRequest,
	) (biz.PageResult[resource.MockResponsePolicy], error)
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
	policies *biz.PolicyUsecase[resource.MockResponsePolicy, resource.MockResponsePolicySpec]
	store    Store
	plugins  *biz.PluginInstallationChecker
}

// NewUsecase 创建模拟响应策略用例。
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

// List 返回满足筛选条件的模拟响应策略。
func (uc *Usecase) List(
	ctx context.Context,
	page biz.PageRequest,
	filter biz.ResourceFilter,
) (biz.PolicyPage[resource.MockResponsePolicy], error) {
	return uc.policies.List(ctx, page, filter)
}

// Get 返回指定模拟响应策略。
func (uc *Usecase) Get(
	ctx context.Context,
	policyID string,
) (biz.PolicyView[resource.MockResponsePolicy], error) {
	return uc.policies.Get(ctx, policyID)
}

// Create 创建模拟响应策略。
func (uc *Usecase) Create(
	ctx context.Context,
	spec resource.MockResponsePolicySpec,
) (biz.PolicyView[resource.MockResponsePolicy], error) {
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return biz.PolicyView[resource.MockResponsePolicy]{}, err
	}
	if err := uc.checkTargetClaimsAvailable(ctx, "", spec); err != nil {
		return biz.PolicyView[resource.MockResponsePolicy]{}, err
	}
	return uc.policies.Create(ctx, spec)
}

// Replace 使用配置版本完整替换模拟响应策略。
func (uc *Usecase) Replace(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
	spec resource.MockResponsePolicySpec,
) (biz.PolicyView[resource.MockResponsePolicy], error) {
	current, err := uc.store.Get(ctx, policyID)
	if err != nil {
		return biz.PolicyView[resource.MockResponsePolicy]{}, err
	}

	if current.Generation != expectedGeneration {
		return biz.PolicyView[resource.MockResponsePolicy]{}, biz.ErrResourceVersionConflict
	}
	if err := uc.checkPluginInstalled(ctx); err != nil {
		return biz.PolicyView[resource.MockResponsePolicy]{}, err
	}
	if err := uc.checkTargetClaimsAvailable(ctx, policyID, spec); err != nil {
		return biz.PolicyView[resource.MockResponsePolicy]{}, err
	}
	return uc.policies.ReplaceObserved(ctx, current, spec)
}

// Delete 使用配置版本删除模拟响应策略。
func (uc *Usecase) Delete(
	ctx context.Context,
	policyID string,
	expectedGeneration int64,
) error {
	return uc.policies.Delete(ctx, policyID, expectedGeneration)
}

func (uc *Usecase) checkPluginInstalled(ctx context.Context) error {
	installed, err := uc.plugins.Installed(ctx, resource.WasmPluginPackageMockResponse)
	if err != nil {
		return err
	}
	if !installed {
		return errors.Conflict(
			adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
			"请先安装模拟响应插件",
		)
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

	return biz.VisitPages(
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
					return false, errors.Conflict(
						adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
						fmt.Sprintf(
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

func policyAttributes(policy *resource.MockResponsePolicy) biz.PolicyAttributes {
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

func policyTargetRefs(spec resource.MockResponsePolicySpec) []resource.PolicyTargetRef {
	return spec.TargetRefs
}
