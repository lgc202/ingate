// Package route 定义 Route 资源在 Generic API Server 中的操作语义。
package route

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	gatewayapi "github.com/lgc202/ingate/pkg/apis/gateway"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayvalidation "github.com/lgc202/ingate/pkg/apis/gateway/validation"
)

var (
	_ rest.RESTCreateStrategy  = strategy{}
	_ rest.RESTUpdateStrategy  = strategy{}
	_ rest.RESTDeleteStrategy  = strategy{}
	_ rest.ResetFieldsStrategy = strategy{}

	_ rest.RESTUpdateStrategy  = statusStrategy{}
	_ rest.ResetFieldsStrategy = statusStrategy{}
)

type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// NewStrategy 创建 Route 主资源的 REST Strategy。
func NewStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

// NamespaceScoped 表示 Route 是集群级资源。
func (strategy) NamespaceScoped() bool {
	return false
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
	route := obj.(*gatewayapi.Route)
	route.Status = gatewayapi.RouteStatus{}
	route.Generation = 1
}

// Validate 校验待创建的 Route。
func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateRoute(obj.(*gatewayapi.Route))
}

// WarningsOnCreate 返回创建 Route 时的警告。
func (strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// Canonicalize 当前不调整 Route 的表示。
func (strategy) Canonicalize(_ runtime.Object) {}

// AllowCreateOnUpdate 禁止通过 Update 创建 Route。
func (strategy) AllowCreateOnUpdate(_ context.Context) bool {
	return false
}

// PrepareForUpdate 隔离 Status，并在 Spec 变化时递增 Generation。
func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*gatewayapi.Route)
	previous := old.(*gatewayapi.Route)

	next.Status = previous.Status
	next.Generation = previous.Generation
	if !apiequality.Semantic.DeepEqual(next.Spec, previous.Spec) {
		next.Generation++
	}
}

// ValidateUpdate 校验待更新的 Route。
func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateRouteUpdate(
		obj.(*gatewayapi.Route),
		old.(*gatewayapi.Route),
	)
}

// WarningsOnUpdate 返回更新 Route 时的警告。
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// AllowUnconditionalUpdate 要求 Route 更新携带 resourceVersion。
func (strategy) AllowUnconditionalUpdate(_ context.Context) bool {
	return false
}

type statusStrategy struct {
	strategy
}

// NewStatusStrategy 创建 Route Status 子资源的 REST Strategy。
func NewStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: NewStrategy(typer)}
}

// GetResetFields 声明 Status 子资源入口不允许修改 Spec。
func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayv1alpha1.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

// PrepareForUpdate 保留已有的 Spec 和 Generation。
func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*gatewayapi.Route)
	previous := old.(*gatewayapi.Route)
	next.Spec = previous.Spec
	next.Generation = previous.Generation
}

// ValidateUpdate 校验 Route Status 子资源更新。
func (statusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateRouteStatusUpdate(
		obj.(*gatewayapi.Route),
		old.(*gatewayapi.Route),
	)
}
