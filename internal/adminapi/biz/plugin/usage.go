package plugin

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/biz/pagination"
	"github.com/lgc202/ingate/internal/adminapi/biz/policy"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// PolicyUsage 表示一条仍依赖已安装插件的策略。
type PolicyUsage struct {
	PolicyID    string
	PolicyKind  resource.Kind
	PolicyType  string
	DisplayName string
}

// UsageFinder 维护强类型 Policy 与执行插件包之间的内部依赖关系。
// 用户只配置业务策略；插件卸载检查不应把 Wasm 包名泄漏到各策略领域。
type UsageFinder struct {
	headerTransformations policy.HeaderTransformationLister
	mockResponses         policy.MockResponseLister
}

// NewUsageFinder 创建插件策略引用查询器。
func NewUsageFinder(
	headerTransformations policy.HeaderTransformationLister,
	mockResponses policy.MockResponseLister,
) *UsageFinder {
	return &UsageFinder{
		headerTransformations: headerTransformations,
		mockResponses:         mockResponses,
	}
}

// ListPolicyUsages 返回仍依赖指定插件包的全部策略。
// 插件详情和卸载校验共用同一份结果，避免控制台与后端对依赖关系作出不同判断。
func (f *UsageFinder) ListPolicyUsages(
	ctx context.Context,
	packageName string,
) ([]PolicyUsage, error) {
	var usages []PolicyUsage
	switch packageName {
	case resource.WasmPluginPackageTransformer:
		err := pagination.VisitPages(
			ctx,
			f.headerTransformations.ListPage,
			func(policy resource.HeaderTransformationPolicy) (bool, error) {
				usages = append(usages, PolicyUsage{
					PolicyID:    policy.Name,
					PolicyKind:  resource.KindHeaderTransformationPolicy,
					PolicyType:  "请求响应转换策略",
					DisplayName: policy.Spec.DisplayName,
				})
				return false, nil
			},
		)
		return usages, err
	case resource.WasmPluginPackageMockResponse:
		err := pagination.VisitPages(ctx, f.mockResponses.ListPage, func(policy resource.MockResponsePolicy) (bool, error) {
			usages = append(usages, PolicyUsage{
				PolicyID:    policy.Name,
				PolicyKind:  resource.KindMockResponsePolicy,
				PolicyType:  "模拟响应策略",
				DisplayName: policy.Spec.DisplayName,
			})
			return false, nil
		})
		return usages, err
	default:
		return usages, nil
	}
}
