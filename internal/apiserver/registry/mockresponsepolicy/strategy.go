package mockresponsepolicy

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
	policy := obj.(*resource.MockResponsePolicy)
	policy.Status = resource.PolicyStatus{}
	canonicalizeSpec(&policy.Spec)
	apiregistry.PrepareObjectMetaForCreate(&policy.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.MockResponsePolicy))
}

func (strategy) Canonicalize(obj runtime.Object) {
	canonicalizeSpec(&obj.(*resource.MockResponsePolicy).Spec)
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.MockResponsePolicy)
	oldPolicy := old.(*resource.MockResponsePolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.MockResponsePolicy))
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.MockResponsePolicy)
	oldPolicy := old.(*resource.MockResponsePolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeSpec(spec *resource.MockResponsePolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.ContentType = strings.TrimSpace(spec.ContentType)
	for i := range spec.TargetRefs {
		spec.TargetRefs[i].Name = strings.TrimSpace(spec.TargetRefs[i].Name)
	}
	for i := range spec.Headers {
		spec.Headers[i].Name = strings.ToLower(strings.TrimSpace(spec.Headers[i].Name))
		spec.Headers[i].Value = strings.TrimSpace(spec.Headers[i].Value)
	}
}
