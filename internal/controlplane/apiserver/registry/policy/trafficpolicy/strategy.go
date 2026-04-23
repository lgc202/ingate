package trafficpolicy

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

type trafficPolicyStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type trafficPolicyStatusStrategy struct{ trafficPolicyStrategy }

var Strategy = trafficPolicyStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = trafficPolicyStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (trafficPolicyStrategy) NamespaceScoped() bool { return false }

func (trafficPolicyStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (trafficPolicyStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"policy.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (trafficPolicyStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	trafficPolicy := obj.(*policyv1alpha1.TrafficPolicy)
	trafficPolicy.Status = policyv1alpha1.TrafficPolicyStatus{}
	trafficPolicy.Generation = 1
}

func (trafficPolicyStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return policyvalidation.ValidateTrafficPolicy(obj.(*policyv1alpha1.TrafficPolicy))
}

func (trafficPolicyStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (trafficPolicyStrategy) Canonicalize(runtime.Object) {}

func (trafficPolicyStrategy) AllowCreateOnUpdate() bool { return false }

func (trafficPolicyStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newTrafficPolicy := obj.(*policyv1alpha1.TrafficPolicy)
	oldTrafficPolicy := old.(*policyv1alpha1.TrafficPolicy)
	newTrafficPolicy.Status = oldTrafficPolicy.Status
	newTrafficPolicy.Generation = oldTrafficPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldTrafficPolicy.Spec, newTrafficPolicy.Spec) {
		newTrafficPolicy.Generation = oldTrafficPolicy.Generation + 1
	}
}

func (trafficPolicyStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return policyvalidation.ValidateTrafficPolicyUpdate(obj.(*policyv1alpha1.TrafficPolicy), old.(*policyv1alpha1.TrafficPolicy))
}

func (trafficPolicyStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (trafficPolicyStrategy) AllowUnconditionalUpdate() bool { return true }

func (trafficPolicyStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"policy.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (trafficPolicyStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newTrafficPolicy := obj.(*policyv1alpha1.TrafficPolicy)
	oldTrafficPolicy := old.(*policyv1alpha1.TrafficPolicy)
	newTrafficPolicy.Spec = oldTrafficPolicy.Spec
	newTrafficPolicy.Generation = oldTrafficPolicy.Generation
}

func (trafficPolicyStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return policyvalidation.ValidateTrafficPolicyStatusUpdate(obj.(*policyv1alpha1.TrafficPolicy), old.(*policyv1alpha1.TrafficPolicy))
}

func (trafficPolicyStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (trafficPolicyStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *policyv1alpha1.TrafficPolicy) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	trafficPolicy, ok := obj.(*policyv1alpha1.TrafficPolicy)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a TrafficPolicy")
	}
	return labels.Set(trafficPolicy.Labels), ToSelectableFields(trafficPolicy), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
