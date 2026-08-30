package certificate

import (
	"context"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	certificateutil "github.com/lgc202/ingate/internal/pkg/certificate"
)

// strategy 定义 Certificate 资源在 API Server 存储前后的处理规则。
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 Certificate status 子资源更新规则。
type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	certificate := obj.(*resource.Certificate)
	certificate.Status = resource.ResourceStatus{}
	canonicalizeCertificateSpec(&certificate.Spec)
	apiregistry.PrepareObjectMetaForCreate(&certificate.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateCertificate(obj.(*resource.Certificate))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*resource.Certificate)
	oldCertificate := old.(*resource.Certificate)

	newCertificate.Status = oldCertificate.Status
	canonicalizeCertificateSpec(&newCertificate.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldCertificate.Spec, newCertificate.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newCertificate.ObjectMeta, &oldCertificate.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateCertificate(obj.(*resource.Certificate))
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*resource.Certificate)
	oldCertificate := old.(*resource.Certificate)

	newCertificate.Spec = oldCertificate.Spec
	metav1.ResetObjectMetaForStatus(&newCertificate.ObjectMeta, &oldCertificate.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	certificate := obj.(*resource.Certificate)
	return apiregistry.ValidateResourceStatus(certificate.Status, certificate.Generation)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func canonicalizeCertificateSpec(spec *resource.CertificateSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.CertificatePEM = certificateutil.NormalizePEM(spec.CertificatePEM)
	spec.PrivateKeyPEM = certificateutil.NormalizePEM(spec.PrivateKeyPEM)
}
