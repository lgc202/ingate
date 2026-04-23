package gateway

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

type gatewayStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type gatewayStatusStrategy struct{ gatewayStrategy }

var Strategy = gatewayStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = gatewayStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (gatewayStrategy) NamespaceScoped() bool { return false }

func (gatewayStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (gatewayStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (gatewayStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	gateway := obj.(*gatewayv1alpha1.Gateway)
	gateway.Status = gatewayv1alpha1.GatewayStatus{}
	gateway.Generation = 1
}

func (gatewayStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateGateway(obj.(*gatewayv1alpha1.Gateway))
}

func (gatewayStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

func (gatewayStrategy) Canonicalize(runtime.Object) {}

func (gatewayStrategy) AllowCreateOnUpdate() bool { return false }

func (gatewayStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGateway := obj.(*gatewayv1alpha1.Gateway)
	oldGateway := old.(*gatewayv1alpha1.Gateway)
	newGateway.Status = oldGateway.Status
	newGateway.Generation = oldGateway.Generation
	if !apiequality.Semantic.DeepEqual(oldGateway.Spec, newGateway.Spec) {
		newGateway.Generation = oldGateway.Generation + 1
	}
}

func (gatewayStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateGatewayUpdate(obj.(*gatewayv1alpha1.Gateway), old.(*gatewayv1alpha1.Gateway))
}

func (gatewayStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string { return nil }

func (gatewayStrategy) AllowUnconditionalUpdate() bool { return true }

func (gatewayStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (gatewayStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newGateway := obj.(*gatewayv1alpha1.Gateway)
	oldGateway := old.(*gatewayv1alpha1.Gateway)
	newGateway.Spec = oldGateway.Spec
	newGateway.Generation = oldGateway.Generation
}

func (gatewayStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateGatewayStatusUpdate(obj.(*gatewayv1alpha1.Gateway), old.(*gatewayv1alpha1.Gateway))
}

func (gatewayStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (gatewayStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *gatewayv1alpha1.Gateway) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	gateway, ok := obj.(*gatewayv1alpha1.Gateway)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a Gateway")
	}
	return labels.Set(gateway.Labels), ToSelectableFields(gateway), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
