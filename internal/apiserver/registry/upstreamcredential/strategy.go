package upstreamcredential

import (
	"context"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	"github.com/lgc202/ingate/internal/pkg/bearer"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 UpstreamCredential 资源在 API Server 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 UpstreamCredential status 子资源更新规则
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
	credential := obj.(*resource.UpstreamCredential)
	credential.Status = resource.ResourceStatus{}
	credential.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateUpstreamCredential(obj.(*resource.UpstreamCredential))
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
	newCredential := obj.(*resource.UpstreamCredential)
	oldCredential := old.(*resource.UpstreamCredential)

	newCredential.Status = oldCredential.Status
	newCredential.Generation = oldCredential.Generation
	if !apiequality.Semantic.DeepEqual(oldCredential.Spec, newCredential.Spec) {
		newCredential.Generation = oldCredential.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateUpstreamCredential(obj.(*resource.UpstreamCredential))
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
	newCredential := obj.(*resource.UpstreamCredential)
	oldCredential := old.(*resource.UpstreamCredential)

	newCredential.Spec = oldCredential.Spec
	metav1.ResetObjectMetaForStatus(&newCredential.ObjectMeta, &oldCredential.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateUpstreamCredential(credential *resource.UpstreamCredential) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}

	if strings.TrimSpace(credential.Spec.DisplayName) == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if credential.Spec.Type == "" {
		errs = append(errs, field.Required(specPath.Child("type"), "type is required"))
		return errs
	}
	if credential.Spec.Type != resource.UpstreamCredentialTypeAPIKey {
		errs = append(errs, field.NotSupported(specPath.Child("type"), credential.Spec.Type, []string{
			string(resource.UpstreamCredentialTypeAPIKey),
		}))
		return errs
	}
	if credential.Spec.APIKey == nil {
		errs = append(errs, field.Required(specPath.Child("apiKey"), "apiKey is required for APIKey credentials"))
		return errs
	}
	value := credential.Spec.APIKey.Value
	if value == "" {
		errs = append(errs, field.Required(specPath.Child("apiKey", "value"), "value is required"))
	} else if !bearer.ValidToken(value) {
		errs = append(errs, field.Invalid(specPath.Child("apiKey", "value"), "<redacted>", "value must be a valid Bearer token"))
	}
	return errs
}
