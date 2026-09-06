// Package mockresponse 处理模拟响应策略的业务规则和资源协作。
package mockresponse

import (
	"context"
	"fmt"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义模拟响应策略管理所需的持久化能力。
type Store interface {
	policy.Store[resource.MockResponsePolicy, resource.MockResponsePolicySpec]
}

// Usecase 协调模拟响应策略的插件、目标、独占范围校验和持久化。
type Usecase struct {
	*policy.Usecase[resource.MockResponsePolicy, resource.MockResponsePolicySpec]
	store   Store
	plugins *plugin.InstallationChecker
}

// NewUsecase 创建模拟响应策略用例。
func NewUsecase(
	store Store,
	routes policy.RouteReader,
	plugins *plugin.InstallationChecker,
) *Usecase {
	uc := &Usecase{
		store:   store,
		plugins: plugins,
	}
	uc.Usecase = policy.NewUsecase(
		store,
		policy.NewRouteTargetResolver(routes),
		policy.ResourceAccessors[resource.MockResponsePolicy, resource.MockResponsePolicySpec]{
			DisplayName: func(item *resource.MockResponsePolicy) string { return item.Spec.DisplayName },
			Enabled:     func(item *resource.MockResponsePolicy) bool { return item.Spec.Enabled },
			Generation:  func(item *resource.MockResponsePolicy) int64 { return item.Generation },
			TargetRefs:  func(item *resource.MockResponsePolicy) []resource.PolicyTargetRef { return item.Spec.TargetRefs },
			TargetRefsFromSpec: func(spec resource.MockResponsePolicySpec) []resource.PolicyTargetRef {
				return spec.TargetRefs
			},
			Status: func(item *resource.MockResponsePolicy) status.Status {
				return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
			},
		},
		uc.validateMutation,
	)
	return uc
}

func (uc *Usecase) validateMutation(
	ctx context.Context,
	excludedPolicyID string,
	spec resource.MockResponsePolicySpec,
) error {
	installed, err := uc.plugins.Installed(ctx, resource.WasmPluginPackageMockResponse)
	if err != nil {
		return err
	}
	if !installed {
		return adminv1.ErrorBusinessRuleViolation("请先安装模拟响应插件")
	}
	return uc.checkTargetClaimsAvailable(ctx, excludedPolicyID, spec)
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
					message := fmt.Sprintf(
						"目标路由已应用模拟响应策略 %q，请先调整其生效范围",
						candidate.Spec.DisplayName,
					)
					return false, adminv1.ErrorResourceConflict("%s", message)
				}
			}
			return false, nil
		},
	)
}
