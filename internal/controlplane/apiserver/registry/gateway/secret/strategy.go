package secret

import (
	"context"
	"fmt"

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

type secretStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

var Strategy = secretStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (secretStrategy) NamespaceScoped() bool { return false }

func (secretStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (secretStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(),
	}
}

func (secretStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {}

func (secretStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateSecret(obj.(*gatewayv1alpha1.Secret))
}

func (secretStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

func (secretStrategy) Canonicalize(runtime.Object) {}

func (secretStrategy) AllowCreateOnUpdate() bool { return false }

func (secretStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {}

func (secretStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateSecretUpdate(obj.(*gatewayv1alpha1.Secret), old.(*gatewayv1alpha1.Secret))
}

func (secretStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string { return nil }

func (secretStrategy) AllowUnconditionalUpdate() bool { return true }

func ToSelectableFields(obj *gatewayv1alpha1.Secret) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	secret, ok := obj.(*gatewayv1alpha1.Secret)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a Secret")
	}
	return labels.Set(secret.Labels), ToSelectableFields(secret), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
