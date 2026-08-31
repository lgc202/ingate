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
	"github.com/lgc202/ingate/internal/pkg/tokenquotaconfig"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
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

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.TokenQuotaPolicy)
	oldPolicy := old.(*resource.TokenQuotaPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	policy := obj.(*resource.TokenQuotaPolicy)
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

func canonicalizeSpec(spec *resource.TokenQuotaPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	if timeZone, _, valid := tokenquotaconfig.LoadLocation(spec.TimeZone); valid {
		spec.TimeZone = timeZone
	} else {
		spec.TimeZone = strings.TrimSpace(spec.TimeZone)
	}
	apiregistry.CanonicalizePolicyTargetRefs(spec.TargetRefs)
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
