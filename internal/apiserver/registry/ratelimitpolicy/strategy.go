package ratelimitpolicy

import (
	"context"

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

const (
	minResponseStatusCode = 400
	maxResponseStatusCode = 599
	maxPluginInteger      = 1<<31 - 1
)

// strategy 定义 RateLimitPolicy 资源在 apiserver 存储前后的处理规则
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
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateRateLimitPolicy(obj.(*resource.RateLimitPolicy))
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
	newPolicy := obj.(*resource.RateLimitPolicy)
	oldPolicy := old.(*resource.RateLimitPolicy)

	newPolicy.Status = oldPolicy.Status
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateRateLimitPolicy(obj.(*resource.RateLimitPolicy))
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

func validateRateLimitPolicy(policy *resource.RateLimitPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, policytarget.ValidateRefs(policy.Spec.TargetRefs, specPath.Child("targetRefs"))...)
	if len(policy.Spec.Rules) == 0 {
		errs = append(errs, field.Required(specPath.Child("rules"), "at least one rule is required"))
	}

	seenRuleNames := make(map[string]bool, len(policy.Spec.Rules))
	for i, rule := range policy.Spec.Rules {
		rulePath := specPath.Child("rules").Index(i)
		if rule.Name == "" {
			errs = append(errs, field.Required(rulePath.Child("name"), "rule name is required"))
		} else if seenRuleNames[rule.Name] {
			errs = append(errs, field.Duplicate(rulePath.Child("name"), rule.Name))
		} else {
			seenRuleNames[rule.Name] = true
		}
		if len(rule.Key.Parts) == 0 {
			errs = append(errs, field.Required(rulePath.Child("key").Child("parts"), "at least one key part is required"))
		}
		for j, part := range rule.Key.Parts {
			errs = append(errs, validateRateLimitKeyPart(part, rulePath.Child("key").Child("parts").Index(j))...)
		}
		if rule.Limit.Requests <= 0 {
			errs = append(errs, field.Invalid(rulePath.Child("limit").Child("requests"), rule.Limit.Requests, "requests must be greater than zero"))
		} else if rule.Limit.Requests > maxPluginInteger {
			errs = append(errs, field.Invalid(rulePath.Child("limit").Child("requests"), rule.Limit.Requests, "requests exceeds the data plane integer range"))
		}
		if rule.Limit.WindowSeconds <= 0 {
			errs = append(errs, field.Invalid(rulePath.Child("limit").Child("windowSeconds"), rule.Limit.WindowSeconds, "windowSeconds must be greater than zero"))
		} else if rule.Limit.WindowSeconds > maxPluginInteger {
			errs = append(errs, field.Invalid(rulePath.Child("limit").Child("windowSeconds"), rule.Limit.WindowSeconds, "windowSeconds exceeds the data plane integer range"))
		}
		if rule.Limit.Burst < 0 {
			errs = append(errs, field.Invalid(rulePath.Child("limit").Child("burst"), rule.Limit.Burst, "burst must not be negative"))
		} else if rule.Limit.Burst > maxPluginInteger {
			errs = append(errs, field.Invalid(rulePath.Child("limit").Child("burst"), rule.Limit.Burst, "burst exceeds the data plane integer range"))
		}
	}

	statusCode := policy.Spec.Response.StatusCode
	if statusCode != 0 && (statusCode < minResponseStatusCode || statusCode > maxResponseStatusCode) {
		errs = append(errs, field.Invalid(specPath.Child("response").Child("statusCode"), statusCode, "statusCode must be between 400 and 599"))
	}
	switch policy.Spec.FailurePolicy {
	case "", resource.RateLimitFailurePolicyFailOpen, resource.RateLimitFailurePolicyFailClose:
	default:
		errs = append(errs, field.NotSupported(specPath.Child("failurePolicy"), policy.Spec.FailurePolicy, []string{
			string(resource.RateLimitFailurePolicyFailOpen),
			string(resource.RateLimitFailurePolicyFailClose),
		}))
	}
	return errs
}

func validateRateLimitKeyPart(part resource.RateLimitKeyPart, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	switch part.Type {
	case resource.RateLimitKeyTypeHeader:
		if part.Name == "" {
			errs = append(errs, field.Required(path.Child("name"), "name is required for Header key parts"))
		} else if messages := k8svalidation.IsHTTPHeaderName(part.Name); len(messages) > 0 {
			errs = append(errs, field.Invalid(path.Child("name"), part.Name, messages[0]))
		}
	case resource.RateLimitKeyTypeQuery,
		resource.RateLimitKeyTypeCookie:
		if part.Name == "" {
			errs = append(errs, field.Required(path.Child("name"), "name is required for this key type"))
		}
	case resource.RateLimitKeyTypeIP,
		resource.RateLimitKeyTypeRoute,
		resource.RateLimitKeyTypeGateway,
		resource.RateLimitKeyTypeRouteRule:
		if part.Name != "" {
			errs = append(errs, field.Invalid(path.Child("name"), part.Name, "name must be empty for this key type"))
		}
	default:
		errs = append(errs, field.NotSupported(path.Child("type"), part.Type, []string{
			string(resource.RateLimitKeyTypeIP),
			string(resource.RateLimitKeyTypeHeader),
			string(resource.RateLimitKeyTypeQuery),
			string(resource.RateLimitKeyTypeCookie),
			string(resource.RateLimitKeyTypeRoute),
			string(resource.RateLimitKeyTypeGateway),
			string(resource.RateLimitKeyTypeRouteRule),
		}))
	}
	return errs
}
