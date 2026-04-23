package authpolicy

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
	policyvalidation "github.com/lgc202/ingate/pkg/apis/policy/validation"
	ingatescheme "github.com/lgc202/ingate/pkg/apis/scheme"
)

type authPolicyStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type authPolicyStatusStrategy struct{ authPolicyStrategy }

var Strategy = authPolicyStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = authPolicyStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (authPolicyStrategy) NamespaceScoped() bool { return false }

func (authPolicyStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (authPolicyStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"policy.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (authPolicyStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	authPolicy := obj.(*policyv1alpha1.AuthPolicy)
	authPolicy.Status = policyv1alpha1.AuthPolicyStatus{}
	authPolicy.Generation = 1
}

func (authPolicyStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return policyvalidation.ValidateAuthPolicy(obj.(*policyv1alpha1.AuthPolicy))
}

func (authPolicyStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (authPolicyStrategy) Canonicalize(runtime.Object) {}

func (authPolicyStrategy) AllowCreateOnUpdate() bool { return false }

func (authPolicyStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newAuthPolicy := obj.(*policyv1alpha1.AuthPolicy)
	oldAuthPolicy := old.(*policyv1alpha1.AuthPolicy)
	newAuthPolicy.Status = oldAuthPolicy.Status
	newAuthPolicy.Generation = oldAuthPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldAuthPolicy.Spec, newAuthPolicy.Spec) {
		newAuthPolicy.Generation = oldAuthPolicy.Generation + 1
	}
}

func (authPolicyStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return policyvalidation.ValidateAuthPolicyUpdate(obj.(*policyv1alpha1.AuthPolicy), old.(*policyv1alpha1.AuthPolicy))
}

func (authPolicyStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (authPolicyStrategy) AllowUnconditionalUpdate() bool { return true }

func (authPolicyStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"policy.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (authPolicyStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newAuthPolicy := obj.(*policyv1alpha1.AuthPolicy)
	oldAuthPolicy := old.(*policyv1alpha1.AuthPolicy)
	newAuthPolicy.Spec = oldAuthPolicy.Spec
	newAuthPolicy.Generation = oldAuthPolicy.Generation
}

func (authPolicyStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return policyvalidation.ValidateAuthPolicyStatusUpdate(obj.(*policyv1alpha1.AuthPolicy), old.(*policyv1alpha1.AuthPolicy))
}

func (authPolicyStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (authPolicyStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *policyv1alpha1.AuthPolicy) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	authPolicy, ok := obj.(*policyv1alpha1.AuthPolicy)
	if !ok {
		return nil, nil, fmt.Errorf("object is not an AuthPolicy")
	}
	return labels.Set(authPolicy.Labels), ToSelectableFields(authPolicy), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
