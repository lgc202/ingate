package iprestrictionpolicy

import (
	"context"
	"maps"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/iprestrictionconfig"
)

// strategy 定义 IPRestrictionPolicy 资源在 API Server 存储前后的处理规则。
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 IPRestrictionPolicy status 子资源更新规则。
type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	policy := obj.(*resource.IPRestrictionPolicy)
	policy.Status = resource.PolicyStatus{}
	canonicalizeSpec(&policy.Spec)
	apiregistry.PrepareObjectMetaForCreate(&policy.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.IPRestrictionPolicy))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.IPRestrictionPolicy)
	oldPolicy := old.(*resource.IPRestrictionPolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.IPRestrictionPolicy))
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.IPRestrictionPolicy)
	oldPolicy := old.(*resource.IPRestrictionPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	policy := obj.(*resource.IPRestrictionPolicy)
	return apiregistry.ValidatePolicyStatus(
		policy.Status,
		policy.Spec.TargetRefs,
		policy.Generation,
	)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func canonicalizeSpec(spec *resource.IPRestrictionPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	apiregistry.CanonicalizePolicyTargetRefs(spec.TargetRefs)
	spec.Allow = canonicalizeRanges(spec.Allow)
	spec.Deny = canonicalizeRanges(spec.Deny)
}

func canonicalizeRanges(values []string) []string {
	unique := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if normalized, valid := iprestrictionconfig.NormalizeRange(value); valid {
			value = normalized
		}
		unique[value] = true
	}
	return slices.Sorted(maps.Keys(unique))
}
