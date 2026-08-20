package certificate

import (
	"context"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

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
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	certificate := obj.(*resource.Certificate)
	certificate.Status = resource.ResourceStatus{}
	certificate.Generation = 1
	canonicalizeCertificateSpec(&certificate.Spec)
	apiregistry.SetUpdatedAt(&certificate.ObjectMeta, certificate.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateCertificate(obj.(*resource.Certificate))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	certificate := obj.(*resource.Certificate)
	canonicalizeCertificateSpec(&certificate.Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newCertificate := obj.(*resource.Certificate)
	oldCertificate := old.(*resource.Certificate)

	newCertificate.Status = oldCertificate.Status
	canonicalizeCertificateSpec(&newCertificate.Spec)
	newCertificate.Generation = oldCertificate.Generation
	if !apiequality.Semantic.DeepEqual(oldCertificate.Spec, newCertificate.Spec) {
		newCertificate.Generation = oldCertificate.Generation + 1
		apiregistry.SetUpdatedAt(&newCertificate.ObjectMeta, time.Now().UTC())
		return
	}
	apiregistry.PreserveUpdatedAt(&newCertificate.ObjectMeta, &oldCertificate.ObjectMeta)
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
		fieldpath.APIVersion(resource.SchemeGroupVersion.String()): fieldpath.NewSet(
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

func canonicalizeCertificateSpec(spec *resource.CertificateSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.CertificatePEM = normalizePEM(spec.CertificatePEM)
	spec.PrivateKeyPEM = normalizePEM(spec.PrivateKeyPEM)
}

func normalizePEM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}
