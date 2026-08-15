package ratelimitpolicy

import (
	"context"
	"maps"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 RateLimitPolicy 资源在 API Server 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 RateLimitPolicy status 子资源更新规则
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
	policy := obj.(*resource.RateLimitPolicy)
	policy.Status = resource.PolicyStatus{}
	policy.Generation = 1
	canonicalizeSpec(&policy.Spec)
	setUpdatedAt(&policy.ObjectMeta, policy.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.RateLimitPolicy))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	policy := obj.(*resource.RateLimitPolicy)
	canonicalizeSpec(&policy.Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Status = oldPolicy.Status
	canonicalizeSpec(&newPolicy.Spec)
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
		setUpdatedAt(&newPolicy.ObjectMeta, time.Now().UTC())
		return
	}
	preserveUpdatedAt(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validatePolicy(obj.(*resource.RateLimitPolicy))
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
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validatePolicy(policy *resource.RateLimitPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, apiregistry.ValidatePolicyTargetRefs(policy.Spec.TargetRefs, specPath.Child("targetRefs"))...)

	subjectPath := specPath.Child("subject")
	switch policy.Spec.Subject.Type {
	case resource.RateLimitSubjectShared, resource.RateLimitSubjectIP:
		if policy.Spec.Subject.HeaderName != "" {
			errs = append(errs, field.Forbidden(subjectPath.Child("headerName"), "headerName is only supported by Header subjects"))
		}
	case resource.RateLimitSubjectHeader:
		if policy.Spec.Subject.HeaderName == "" {
			errs = append(errs, field.Required(subjectPath.Child("headerName"), "headerName is required for Header subjects"))
		} else if messages := k8svalidation.IsHTTPHeaderName(policy.Spec.Subject.HeaderName); len(messages) > 0 {
			errs = append(errs, field.Invalid(subjectPath.Child("headerName"), policy.Spec.Subject.HeaderName, messages[0]))
		}
	default:
		errs = append(errs, field.NotSupported(subjectPath.Child("type"), policy.Spec.Subject.Type, []string{
			string(resource.RateLimitSubjectShared),
			string(resource.RateLimitSubjectIP),
			string(resource.RateLimitSubjectHeader),
		}))
	}

	limitPath := specPath.Child("limit")
	if policy.Spec.Limit.Requests <= 0 || policy.Spec.Limit.Requests > resource.RateLimitMaxRequests {
		errs = append(errs, field.Invalid(limitPath.Child("requests"), policy.Spec.Limit.Requests, "requests is outside the supported range"))
	}
	if policy.Spec.Limit.WindowSeconds <= 0 || policy.Spec.Limit.WindowSeconds > resource.RateLimitMaxWindowSeconds {
		errs = append(errs, field.Invalid(limitPath.Child("windowSeconds"), policy.Spec.Limit.WindowSeconds, "windowSeconds is outside the supported range"))
	}
	return errs
}

func canonicalizeSpec(spec *resource.RateLimitPolicySpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.Subject.HeaderName = strings.ToLower(strings.TrimSpace(spec.Subject.HeaderName))
	for i := range spec.TargetRefs {
		spec.TargetRefs[i].Name = strings.TrimSpace(spec.TargetRefs[i].Name)
	}
}

func setUpdatedAt(metadata *metav1.ObjectMeta, updatedAt time.Time) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt.Format(time.RFC3339Nano)
}

func preserveUpdatedAt(metadata, oldMetadata *metav1.ObjectMeta) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	delete(metadata.Annotations, resource.AnnotationUpdatedAt)
	updatedAt := oldMetadata.Annotations[resource.AnnotationUpdatedAt]
	if updatedAt == "" {
		return
	}
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt
}
