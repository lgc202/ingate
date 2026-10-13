// Package httproute 定义 HTTPRoute 资源在 Generic API Server 中的操作语义。
package httproute

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayvalidation "github.com/lgc202/ingate/pkg/apis/gateway/validation"
)

type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type statusStrategy struct {
	strategy
}

var (
	_ rest.RESTCreateStrategy  = strategy{}
	_ rest.RESTUpdateStrategy  = strategy{}
	_ rest.RESTDeleteStrategy  = strategy{}
	_ rest.ResetFieldsStrategy = strategy{}

	_ rest.RESTUpdateStrategy  = statusStrategy{}
	_ rest.ResetFieldsStrategy = statusStrategy{}
)

// NewStrategy 创建 HTTPRoute 主资源的 REST Strategy。
func NewStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

// NewStatusStrategy 创建 HTTPRoute Status 子资源的 REST Strategy。
func NewStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: NewStrategy(typer)}
}

// NamespaceScoped 表示 HTTPRoute 是命名空间级资源。
func (strategy) NamespaceScoped() bool {
	return true
}

// GetResetFields 声明主资源入口不允许修改 Status。
func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayv1alpha1.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

// PrepareForCreate 清空调用方提交的 Status，并初始化 Generation。
func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	route := obj.(*gatewayapi.HTTPRoute)
	route.Status = gatewayapi.HTTPRouteStatus{}
	route.Generation = 1
}

// Validate 校验待创建的 HTTPRoute。
func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateHTTPRoute(obj.(*gatewayapi.HTTPRoute))
}

// WarningsOnCreate 返回创建 HTTPRoute 时的警告。
func (strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// Canonicalize 当前不调整 HTTPRoute 的表示。
func (strategy) Canonicalize(_ runtime.Object) {}

// AllowCreateOnUpdate 禁止通过 Update 创建 HTTPRoute。
func (strategy) AllowCreateOnUpdate(_ context.Context) bool {
	return false
}

// PrepareForUpdate 隔离 Status，并在 Spec 变化时递增 Generation。
func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*gatewayapi.HTTPRoute)
	previous := old.(*gatewayapi.HTTPRoute)

	next.Status = previous.Status
	next.Generation = previous.Generation
	if !apiequality.Semantic.DeepEqual(next.Spec, previous.Spec) {
		next.Generation++
	}
}

// ValidateUpdate 校验待更新的 HTTPRoute。
func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateHTTPRouteUpdate(
		obj.(*gatewayapi.HTTPRoute),
		old.(*gatewayapi.HTTPRoute),
	)
}

// WarningsOnUpdate 返回更新 HTTPRoute 时的警告。
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// AllowUnconditionalUpdate 要求 HTTPRoute 更新携带 resourceVersion。
func (strategy) AllowUnconditionalUpdate(_ context.Context) bool {
	return false
}

// GetResetFields 声明 Status 子资源入口不允许修改 Spec。
func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayv1alpha1.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

// PrepareForUpdate 保留已有的 Spec 和非 Status 元数据。
func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*gatewayapi.HTTPRoute)
	previous := old.(*gatewayapi.HTTPRoute)
	next.Spec = previous.Spec
	metav1.ResetObjectMetaForStatus(&next.ObjectMeta, &previous.ObjectMeta)
}

// ValidateUpdate 校验 HTTPRoute Status 子资源更新。
func (statusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateHTTPRouteStatusUpdate(
		obj.(*gatewayapi.HTTPRoute),
		old.(*gatewayapi.HTTPRoute),
	)
}
