package headertransformationpolicy

import (
	"context"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	policy := obj.(*resource.HeaderTransformationPolicy)
	policy.Status = resource.PolicyStatus{}
	canonicalizeSpec(&policy.Spec)
	apiregistry.PrepareObjectMetaForCreate(&policy.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.HeaderTransformationPolicy))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.HeaderTransformationPolicy)
	oldPolicy := old.(*resource.HeaderTransformationPolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.HeaderTransformationPolicy))
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.HeaderTransformationPolicy)
	oldPolicy := old.(*resource.HeaderTransformationPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	policy := obj.(*resource.HeaderTransformationPolicy)
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

func canonicalizeSpec(spec *resource.HeaderTransformationPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	apiregistry.CanonicalizePolicyTargetRefs(spec.TargetRefs)
	canonicalizeRules(spec.RequestRules)
	canonicalizeRules(spec.ResponseRules)
}

func canonicalizeRules(rules []resource.HeaderTransformationRule) {
	for i := range rules {
		rules[i].Name = httpheader.NormalizeName(rules[i].Name)
		rules[i].Value = httpheader.NormalizeValue(rules[i].Value)
		if rules[i].Operation == resource.HeaderTransformationRename {
			rules[i].Value = httpheader.NormalizeName(rules[i].Value)
		}
	}
}
