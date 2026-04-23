package resolvedgateway

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

	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	gatewayvalidation "github.com/lgc202/ingate/pkg/apis/gateway/validation"
	ingatescheme "github.com/lgc202/ingate/pkg/apis/scheme"
)

type resolvedGatewayStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type resolvedGatewayStatusStrategy struct{ resolvedGatewayStrategy }

var Strategy = resolvedGatewayStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = resolvedGatewayStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (resolvedGatewayStrategy) NamespaceScoped() bool { return false }

func (resolvedGatewayStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (resolvedGatewayStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (resolvedGatewayStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	resolvedGateway := obj.(*gatewayv1alpha1.ResolvedGateway)
	resolvedGateway.Status = gatewayv1alpha1.ResolvedGatewayStatus{}
	resolvedGateway.Generation = 1
}

func (resolvedGatewayStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateResolvedGateway(obj.(*gatewayv1alpha1.ResolvedGateway))
}

func (resolvedGatewayStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (resolvedGatewayStrategy) Canonicalize(runtime.Object) {}

func (resolvedGatewayStrategy) AllowCreateOnUpdate() bool { return false }

func (resolvedGatewayStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newResolvedGateway := obj.(*gatewayv1alpha1.ResolvedGateway)
	oldResolvedGateway := old.(*gatewayv1alpha1.ResolvedGateway)
	newResolvedGateway.Status = oldResolvedGateway.Status
	newResolvedGateway.Generation = oldResolvedGateway.Generation
	if !apiequality.Semantic.DeepEqual(oldResolvedGateway.Spec, newResolvedGateway.Spec) {
		newResolvedGateway.Generation = oldResolvedGateway.Generation + 1
	}
}

func (resolvedGatewayStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateResolvedGatewayUpdate(obj.(*gatewayv1alpha1.ResolvedGateway), old.(*gatewayv1alpha1.ResolvedGateway))
}

func (resolvedGatewayStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (resolvedGatewayStrategy) AllowUnconditionalUpdate() bool { return true }

func (resolvedGatewayStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (resolvedGatewayStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newResolvedGateway := obj.(*gatewayv1alpha1.ResolvedGateway)
	oldResolvedGateway := old.(*gatewayv1alpha1.ResolvedGateway)
	newResolvedGateway.Spec = oldResolvedGateway.Spec
	newResolvedGateway.Generation = oldResolvedGateway.Generation
}

func (resolvedGatewayStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateResolvedGatewayStatusUpdate(obj.(*gatewayv1alpha1.ResolvedGateway), old.(*gatewayv1alpha1.ResolvedGateway))
}

func (resolvedGatewayStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (resolvedGatewayStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *gatewayv1alpha1.ResolvedGateway) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	resolvedGateway, ok := obj.(*gatewayv1alpha1.ResolvedGateway)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a ResolvedGateway")
	}
	return labels.Set(resolvedGateway.Labels), ToSelectableFields(resolvedGateway), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
