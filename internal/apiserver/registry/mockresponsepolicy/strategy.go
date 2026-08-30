package mockresponsepolicy

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
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/mockresponseconfig"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
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

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.MockResponsePolicy)
	oldPolicy := old.(*resource.MockResponsePolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	policy := obj.(*resource.MockResponsePolicy)
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

func canonicalizeSpec(spec *resource.MockResponsePolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	if contentType, valid := mockresponseconfig.NormalizeContentType(spec.ContentType); valid {
		spec.ContentType = contentType
	} else {
		spec.ContentType = strings.TrimSpace(spec.ContentType)
	}
	apiregistry.CanonicalizePolicyTargetRefs(spec.TargetRefs)
	for i := range spec.Headers {
		spec.Headers[i].Name = httpheader.NormalizeName(spec.Headers[i].Name)
		spec.Headers[i].Value = httpheader.NormalizeValue(spec.Headers[i].Value)
	}
	slices.SortFunc(spec.Headers, func(left, right resource.HeaderValue) int {
		return strings.Compare(left.Name, right.Name)
	})
}
