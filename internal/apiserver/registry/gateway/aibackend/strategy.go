// Package aibackend 定义 AIBackend 资源在 Generic API Server 中的操作语义。
package aibackend

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

// NewStrategy 创建 AIBackend 主资源的 REST Strategy。
func NewStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

// NewStatusStrategy 创建 AIBackend Status 子资源的 REST Strategy。
func NewStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: NewStrategy(typer)}
}

// NamespaceScoped 表示 AIBackend 是集群级资源。
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
	backend := obj.(*gatewayapi.AIBackend)
	backend.Status = gatewayapi.AIBackendStatus{}
	backend.Generation = 1
}

// Validate 校验待创建的 AIBackend。
func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateAIBackend(obj.(*gatewayapi.AIBackend))
}

// WarningsOnCreate 返回创建 AIBackend 时的警告。
func (strategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// Canonicalize 保留已校验的 API 格式及路径前缀，不改写显式配置。
func (strategy) Canonicalize(_ runtime.Object) {}

// AllowCreateOnUpdate 禁止通过 Update 创建 AIBackend。
func (strategy) AllowCreateOnUpdate(_ context.Context) bool {
	return false
}

// PrepareForUpdate 隔离 Status，并在 Spec 变化时递增 Generation。
func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	next := obj.(*gatewayapi.AIBackend)
	previous := old.(*gatewayapi.AIBackend)

	next.Status = previous.Status
	next.Generation = previous.Generation
	if !apiequality.Semantic.DeepEqual(next.Spec, previous.Spec) {
		next.Generation++
	}
}

// ValidateUpdate 校验待更新的 AIBackend。
func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateAIBackendUpdate(
		obj.(*gatewayapi.AIBackend),
		old.(*gatewayapi.AIBackend),
	)
}

// WarningsOnUpdate 返回更新 AIBackend 时的警告。
func (strategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

// AllowUnconditionalUpdate 要求 AIBackend 更新携带 resourceVersion。
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
	next := obj.(*gatewayapi.AIBackend)
	previous := old.(*gatewayapi.AIBackend)
	next.Spec = previous.Spec
	metav1.ResetObjectMetaForStatus(&next.ObjectMeta, &previous.ObjectMeta)
}

// ValidateUpdate 校验 AIBackend Status 子资源更新。
func (statusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateAIBackendStatusUpdate(
		obj.(*gatewayapi.AIBackend),
		old.(*gatewayapi.AIBackend),
	)
}
