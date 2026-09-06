// Package headertransformation 处理请求响应 Header 转换策略的业务规则和资源协作。
package headertransformation

import (
	"context"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz/plugin"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	"github.com/lgc202/ingate/internal/adminapi/biz/resource/status"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// Store 定义请求响应 Header 转换策略管理所需的持久化能力。
type Store interface {
	policy.Store[resource.HeaderTransformationPolicy, resource.HeaderTransformationPolicySpec]
}

// Usecase 协调请求响应 Header 转换策略的插件、目标校验和持久化。
type Usecase struct {
	*policy.Usecase[resource.HeaderTransformationPolicy, resource.HeaderTransformationPolicySpec]
	plugins *plugin.InstallationChecker
}

// NewUsecase 创建请求响应 Header 转换策略用例。
func NewUsecase(
	store Store,
	routes policy.RouteReader,
	plugins *plugin.InstallationChecker,
) *Usecase {
	uc := &Usecase{plugins: plugins}
	uc.Usecase = policy.NewUsecase(
		store,
		policy.NewRouteTargetResolver(routes),
		policy.ResourceAccessors[resource.HeaderTransformationPolicy, resource.HeaderTransformationPolicySpec]{
			DisplayName: func(item *resource.HeaderTransformationPolicy) string { return item.Spec.DisplayName },
			Enabled:     func(item *resource.HeaderTransformationPolicy) bool { return item.Spec.Enabled },
			Generation:  func(item *resource.HeaderTransformationPolicy) int64 { return item.Generation },
			TargetRefs: func(item *resource.HeaderTransformationPolicy) []resource.PolicyTargetRef {
				return item.Spec.TargetRefs
			},
			TargetRefsFromSpec: func(spec resource.HeaderTransformationPolicySpec) []resource.PolicyTargetRef {
				return spec.TargetRefs
			},
			Status: func(item *resource.HeaderTransformationPolicy) status.Status {
				return policy.Status(item.Generation, item.Spec.Enabled, len(item.Spec.TargetRefs), item.Status.Conditions)
			},
		},
		uc.validateMutation,
	)
	return uc
}

func (uc *Usecase) validateMutation(
	ctx context.Context,
	_ string,
	_ resource.HeaderTransformationPolicySpec,
) error {
	installed, err := uc.plugins.Installed(ctx, resource.WasmPluginPackageTransformer)
	if err != nil {
		return err
	}
	if !installed {
		return adminv1.ErrorBusinessRuleViolation("请先安装请求响应转换插件")
	}
	return nil
}
