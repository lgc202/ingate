package tokenquotapolicy

import (
	"context"
	"fmt"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	"github.com/lgc202/ingate/internal/apiserver/registry/policytarget"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 TokenQuotaPolicy 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 TokenQuotaPolicy status 子资源更新规则
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
	policy := obj.(*resource.TokenQuotaPolicy)
	policy.Status = resource.PolicyStatus{}
	policy.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateTokenQuotaPolicy(obj.(*resource.TokenQuotaPolicy))
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
	newPolicy := obj.(*resource.TokenQuotaPolicy)
	oldPolicy := old.(*resource.TokenQuotaPolicy)

	newPolicy.Status = oldPolicy.Status
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateTokenQuotaPolicy(obj.(*resource.TokenQuotaPolicy))
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
	newPolicy := obj.(*resource.TokenQuotaPolicy)
	oldPolicy := old.(*resource.TokenQuotaPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateTokenQuotaPolicy(policy *resource.TokenQuotaPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}
	if strings.TrimSpace(policy.Spec.DisplayName) == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, policytarget.ValidateRefs(policy.Spec.TargetRefs, specPath.Child("targetRefs"))...)
	errs = append(errs, validateTokenQuotaSubject(policy.Spec.Subject, specPath.Child("subject"))...)
	if policy.Spec.Quota.Tokens <= 0 {
		errs = append(errs, field.Invalid(specPath.Child("quota").Child("tokens"), policy.Spec.Quota.Tokens, "tokens must be greater than zero"))
	} else if policy.Spec.Quota.Tokens > resource.TokenQuotaMaxTokens {
		errs = append(errs, field.Invalid(
			specPath.Child("quota").Child("tokens"),
			policy.Spec.Quota.Tokens,
			fmt.Sprintf("tokens must be less than or equal to %d", resource.TokenQuotaMaxTokens),
		))
	}
	if policy.Spec.Quota.WindowSeconds <= 0 {
		errs = append(errs, field.Invalid(specPath.Child("quota").Child("windowSeconds"), policy.Spec.Quota.WindowSeconds, "windowSeconds must be greater than zero"))
	} else if policy.Spec.Quota.WindowSeconds > resource.TokenQuotaMaxWindowSeconds {
		errs = append(errs, field.Invalid(
			specPath.Child("quota").Child("windowSeconds"),
			policy.Spec.Quota.WindowSeconds,
			fmt.Sprintf("windowSeconds must be less than or equal to %d", resource.TokenQuotaMaxWindowSeconds),
		))
	}
	switch policy.Spec.FailurePolicy {
	case "":
		errs = append(errs, field.Required(specPath.Child("failurePolicy"), "failurePolicy is required"))
	case resource.TokenQuotaFailurePolicyFailOpen, resource.TokenQuotaFailurePolicyFailClose:
	default:
		errs = append(errs, field.NotSupported(specPath.Child("failurePolicy"), policy.Spec.FailurePolicy, []string{
			string(resource.TokenQuotaFailurePolicyFailOpen),
			string(resource.TokenQuotaFailurePolicyFailClose),
		}))
	}
	return errs
}

func validateTokenQuotaSubject(subject resource.TokenQuotaSubject, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	switch subject.Type {
	case "":
		errs = append(errs, field.Required(path.Child("type"), "subject type is required"))
	case resource.TokenQuotaSubjectTypeHeader:
		if subject.HeaderName == "" {
			errs = append(errs, field.Required(path.Child("headerName"), "headerName is required for Header subjects"))
		} else if messages := k8svalidation.IsHTTPHeaderName(subject.HeaderName); len(messages) > 0 {
			errs = append(errs, field.Invalid(path.Child("headerName"), subject.HeaderName, messages[0]))
		}
	case resource.TokenQuotaSubjectTypeShared, resource.TokenQuotaSubjectTypeIP:
		if subject.HeaderName != "" {
			errs = append(errs, field.Invalid(path.Child("headerName"), subject.HeaderName, "headerName must be empty for this subject type"))
		}
	default:
		errs = append(errs, field.NotSupported(path.Child("type"), subject.Type, []string{
			string(resource.TokenQuotaSubjectTypeShared),
			string(resource.TokenQuotaSubjectTypeIP),
			string(resource.TokenQuotaSubjectTypeHeader),
		}))
	}
	return errs
}
