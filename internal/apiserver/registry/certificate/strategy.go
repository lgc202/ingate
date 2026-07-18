package certificate

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	certificateparser "github.com/lgc202/ingate/internal/pkg/certificate"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 Certificate 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 Certificate status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	certificate := obj.(*resource.Certificate)
	certificate.Status = resource.ResourceStatus{}
	certificate.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateCertificate(obj.(*resource.Certificate))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*resource.Certificate)
	oldCertificate := old.(*resource.Certificate)

	newCertificate.Status = oldCertificate.Status
	newCertificate.Generation = oldCertificate.Generation
	if !apiequality.Semantic.DeepEqual(oldCertificate.Spec, newCertificate.Spec) {
		newCertificate.Generation = oldCertificate.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateCertificate(obj.(*resource.Certificate))
}

func (strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*resource.Certificate)
	oldCertificate := old.(*resource.Certificate)

	newCertificate.Spec = oldCertificate.Spec
	metav1.ResetObjectMetaForStatus(&newCertificate.ObjectMeta, &oldCertificate.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateCertificate(certificate *resource.Certificate) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}

	if certificate.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if certificate.Spec.CertificatePEM == "" {
		errs = append(errs, field.Required(specPath.Child("certificatePEM"), "certificatePEM is required"))
	}
	if certificate.Spec.PrivateKeyPEM == "" {
		errs = append(errs, field.Required(specPath.Child("privateKeyPEM"), "privateKeyPEM is required"))
	}
	if certificate.Spec.CertificatePEM == "" || certificate.Spec.PrivateKeyPEM == "" {
		return errs
	}

	if _, err := certificateparser.ParseKeyPair(certificate.Spec.CertificatePEM, certificate.Spec.PrivateKeyPEM); err != nil {
		errs = append(errs, field.Invalid(
			specPath.Child("certificatePEM"),
			"<redacted>",
			"certificate and private key must be valid PEM and match",
		))
	}
	return errs
}
