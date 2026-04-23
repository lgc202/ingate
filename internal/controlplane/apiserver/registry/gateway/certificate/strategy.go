package certificate

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

type certificateStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

type certificateStatusStrategy struct{ certificateStrategy }

var Strategy = certificateStrategy{ingatescheme.Scheme, names.SimpleNameGenerator}
var StatusStrategy = certificateStatusStrategy{Strategy}

var (
	_ rest.RESTCreateStrategy              = Strategy
	_ rest.RESTUpdateStrategy              = Strategy
	_ rest.RESTDeleteStrategy              = Strategy
	_ rest.GarbageCollectionDeleteStrategy = Strategy
)

func (certificateStrategy) NamespaceScoped() bool { return false }

func (certificateStrategy) DefaultGarbageCollectionPolicy(context.Context) rest.GarbageCollectionPolicy {
	return rest.DeleteDependents
}

func (certificateStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("status")),
	}
}

func (certificateStrategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	certificate := obj.(*gatewayv1alpha1.Certificate)
	certificate.Status = gatewayv1alpha1.CertificateStatus{}
	certificate.Generation = 1
}

func (certificateStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateCertificate(obj.(*gatewayv1alpha1.Certificate))
}

func (certificateStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string { return nil }

func (certificateStrategy) Canonicalize(runtime.Object) {}

func (certificateStrategy) AllowCreateOnUpdate() bool { return false }

func (certificateStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*gatewayv1alpha1.Certificate)
	oldCertificate := old.(*gatewayv1alpha1.Certificate)
	newCertificate.Status = oldCertificate.Status
	newCertificate.Generation = oldCertificate.Generation
	if !apiequality.Semantic.DeepEqual(oldCertificate.Spec, newCertificate.Spec) {
		newCertificate.Generation = oldCertificate.Generation + 1
	}
}

func (certificateStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateCertificateUpdate(obj.(*gatewayv1alpha1.Certificate), old.(*gatewayv1alpha1.Certificate))
}

func (certificateStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (certificateStrategy) AllowUnconditionalUpdate() bool { return true }

func (certificateStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"gateway.ingate.io/v1alpha1": fieldpath.NewSet(fieldpath.MakePathOrDie("spec")),
	}
}

func (certificateStatusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*gatewayv1alpha1.Certificate)
	oldCertificate := old.(*gatewayv1alpha1.Certificate)
	newCertificate.Spec = oldCertificate.Spec
	newCertificate.Generation = oldCertificate.Generation
}

func (certificateStatusStrategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	return gatewayvalidation.ValidateCertificateStatusUpdate(obj.(*gatewayv1alpha1.Certificate), old.(*gatewayv1alpha1.Certificate))
}

func (certificateStatusStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func (certificateStatusStrategy) Canonicalize(runtime.Object) {}

func ToSelectableFields(obj *gatewayv1alpha1.Certificate) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	certificate, ok := obj.(*gatewayv1alpha1.Certificate)
	if !ok {
		return nil, nil, fmt.Errorf("object is not a Certificate")
	}
	return labels.Set(certificate.Labels), ToSelectableFields(certificate), nil
}

func Matcher(label labels.Selector, fieldSelector fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: fieldSelector, GetAttrs: GetAttrs}
}
