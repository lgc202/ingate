package biz

import (
	"context"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
)

// PluginPolicyUsage 表示一条仍依赖已安装插件的策略
type PluginPolicyUsage struct {
	PolicyType  string
	DisplayName string
}

// PluginUsageFinder 维护强类型 Policy 与执行插件包之间的内部依赖关系
// 用户只配置业务策略；插件卸载检查不应把 Wasm 包名泄漏到各策略领域
type PluginUsageFinder struct {
	headerTransformations HeaderTransformationPolicyLister
	mockResponses         MockResponsePolicyLister
}

// NewPluginUsageFinder 创建插件策略引用查询器
func NewPluginUsageFinder(
	headerTransformations HeaderTransformationPolicyLister,
	mockResponses MockResponsePolicyLister,
) *PluginUsageFinder {
	return &PluginUsageFinder{
		headerTransformations: headerTransformations,
		mockResponses:         mockResponses,
	}
}

// Find 返回第一条仍依赖指定插件包的策略
func (f *PluginUsageFinder) Find(
	ctx context.Context,
	packageName string,
) (*PluginPolicyUsage, error) {
	switch packageName {
	case resource.WasmPluginPackageTransformer:
		var usage *PluginPolicyUsage
		err := VisitPages(ctx, f.headerTransformations.ListPage, func(policy resource.HeaderTransformationPolicy) (bool, error) {
			usage = &PluginPolicyUsage{PolicyType: "请求响应转换策略", DisplayName: policy.Spec.DisplayName}
			return true, nil
		})
		return usage, err
	case resource.WasmPluginPackageMockResponse:
		var usage *PluginPolicyUsage
		err := VisitPages(ctx, f.mockResponses.ListPage, func(policy resource.MockResponsePolicy) (bool, error) {
			usage = &PluginPolicyUsage{PolicyType: "模拟响应策略", DisplayName: policy.Spec.DisplayName}
			return true, nil
		})
		return usage, err
	default:
		return nil, nil
	}
}
