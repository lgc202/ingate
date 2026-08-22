package tokenquotapolicy

import (
	"context"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	policy := obj.(*resource.TokenQuotaPolicy)
	policy.Status = resource.PolicyStatus{}
	canonicalizeSpec(&policy.Spec)
	apiregistry.PrepareObjectMetaForCreate(&policy.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.TokenQuotaPolicy))
}

func (strategy) Canonicalize(obj runtime.Object) {
	canonicalizeSpec(&obj.(*resource.TokenQuotaPolicy).Spec)
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.TokenQuotaPolicy)
	oldPolicy := old.(*resource.TokenQuotaPolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.TokenQuotaPolicy))
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.TokenQuotaPolicy)
	oldPolicy := old.(*resource.TokenQuotaPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeSpec(spec *resource.TokenQuotaPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.TimeZone = strings.TrimSpace(spec.TimeZone)
	for i := range spec.TargetRefs {
		spec.TargetRefs[i].Name = strings.TrimSpace(spec.TargetRefs[i].Name)
	}
	slices.SortFunc(spec.Limits, func(a, b resource.TokenQuotaLimit) int {
		return tokenQuotaPeriodOrder(a.Period) - tokenQuotaPeriodOrder(b.Period)
	})
}

func tokenQuotaPeriodOrder(period resource.TokenQuotaPeriod) int {
	switch period {
	case resource.TokenQuotaPeriodDay:
		return 1
	case resource.TokenQuotaPeriodWeek:
		return 2
	case resource.TokenQuotaPeriodMonth:
		return 3
	default:
		return 4
	}
}
