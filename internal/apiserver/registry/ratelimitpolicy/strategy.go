package ratelimitpolicy

import (
	"context"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

// strategy 定义 RateLimitPolicy 资源在 API Server 存储前后的处理规则
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
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	policy := obj.(*resource.RateLimitPolicy)
	policy.Status = resource.PolicyStatus{}
	policy.Generation = 1
	canonicalizeSpec(&policy.Spec)
	apiregistry.SetUpdatedAt(&policy.ObjectMeta, policy.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.RateLimitPolicy))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	policy := obj.(*resource.RateLimitPolicy)
	canonicalizeSpec(&policy.Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
		apiregistry.SetUpdatedAt(&newPolicy.ObjectMeta, time.Now().UTC())
		return
	}
	apiregistry.PreserveUpdatedAt(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.RateLimitPolicy))
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
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeSpec(spec *resource.RateLimitPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.Subject.HeaderName = strings.ToLower(strings.TrimSpace(spec.Subject.HeaderName))
	for i := range spec.TargetRefs {
		spec.TargetRefs[i].Name = strings.TrimSpace(spec.TargetRefs[i].Name)
	}
}
