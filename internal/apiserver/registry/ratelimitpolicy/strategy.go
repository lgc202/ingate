package ratelimitpolicy

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 RateLimitPolicy 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 RateLimitPolicy status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	policy := obj.(*resource.RateLimitPolicy)
	policy.Status = resource.ResourceStatus{}
	policy.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return nil
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Status = oldPolicy.Status
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func (strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Spec = oldPolicy.Spec
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}
