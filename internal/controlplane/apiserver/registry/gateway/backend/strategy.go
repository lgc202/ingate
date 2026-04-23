package backend

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

type backendStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type backendStatusStrategy struct{ backendStrategy }

var Strategy = backendStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = backendStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (backendStrategy) NamespaceScoped() bool { return false }

func (backendStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (backendStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (backendStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	backend := obj.(*gatewayv1alpha1.Backend)
	backend.Status = gatewayv1alpha1.BackendStatus{}
	backend.Generation = 1
}

func (backendStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateBackend(obj.(*gatewayv1alpha1.Backend))
}

func (backendStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

func (backendStrategy) Canonicalize(runtime.Object) {}

func (backendStrategy) AllowCreateOnUpdate() bool { return false }

func (backendStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBackend := obj.(*gatewayv1alpha1.Backend)
	oldBackend := old.(*gatewayv1alpha1.Backend)
	newBackend.Status = oldBackend.Status
	newBackend.Generation = oldBackend.Generation
	if !apiequality.Semantic.DeepEqual(oldBackend.Spec, newBackend.Spec) {
		newBackend.Generation = oldBackend.Generation + 1
	}
}

func (backendStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateBackendUpdate(obj.(*gatewayv1alpha1.Backend), old.(*gatewayv1alpha1.Backend))
}

func (backendStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (backendStrategy) AllowUnconditionalUpdate() bool { return true }

func (backendStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (backendStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newBackend := obj.(*gatewayv1alpha1.Backend)
	oldBackend := old.(*gatewayv1alpha1.Backend)
	newBackend.Spec = oldBackend.Spec
	newBackend.Generation = oldBackend.Generation
}

func (backendStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateBackendStatusUpdate(obj.(*gatewayv1alpha1.Backend), old.(*gatewayv1alpha1.Backend))
}

func (backendStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (backendStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *gatewayv1alpha1.Backend) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	backend, ok := obj.(*gatewayv1alpha1.Backend)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a Backend")
	}
	return labels.Set(backend.Labels), ToSelectableFields(backend), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
