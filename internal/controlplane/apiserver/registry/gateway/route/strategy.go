package route

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

type routeStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type routeStatusStrategy struct{ routeStrategy }

var Strategy = routeStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = routeStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (routeStrategy) NamespaceScoped() bool { return false }

func (routeStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (routeStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (routeStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	route := obj.(*gatewayv1alpha1.Route)
	route.Status = gatewayv1alpha1.RouteStatus{}
	route.Generation = 1
}

func (routeStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateRoute(obj.(*gatewayv1alpha1.Route))
}

func (routeStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

func (routeStrategy) Canonicalize(runtime.Object) {}

func (routeStrategy) AllowCreateOnUpdate() bool { return false }

func (routeStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newRoute := obj.(*gatewayv1alpha1.Route)
	oldRoute := old.(*gatewayv1alpha1.Route)
	newRoute.Status = oldRoute.Status
	newRoute.Generation = oldRoute.Generation
	if !apiequality.Semantic.DeepEqual(oldRoute.Spec, newRoute.Spec) {
		newRoute.Generation = oldRoute.Generation + 1
	}
}

func (routeStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateRouteUpdate(obj.(*gatewayv1alpha1.Route), old.(*gatewayv1alpha1.Route))
}

func (routeStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string { return nil }

func (routeStrategy) AllowUnconditionalUpdate() bool { return true }

func (routeStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (routeStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newRoute := obj.(*gatewayv1alpha1.Route)
	oldRoute := old.(*gatewayv1alpha1.Route)
	newRoute.Spec = oldRoute.Spec
	newRoute.Generation = oldRoute.Generation
}

func (routeStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateRouteStatusUpdate(obj.(*gatewayv1alpha1.Route), old.(*gatewayv1alpha1.Route))
}

func (routeStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (routeStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *gatewayv1alpha1.Route) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	route, ok := obj.(*gatewayv1alpha1.Route)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a Route")
	}
	return labels.Set(route.Labels), ToSelectableFields(route), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
