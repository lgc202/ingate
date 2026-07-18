package accesscontrolpolicy

import (
	"context"
	"net/netip"

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
)

// strategy 定义 AccessControlPolicy 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 AccessControlPolicy status 子资源更新规则
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
	policy := obj.(*resource.AccessControlPolicy)
	policy.Status = resource.PolicyStatus{}
	policy.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateAccessControlPolicy(obj.(*resource.AccessControlPolicy))
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
	newPolicy := obj.(*resource.AccessControlPolicy)
	oldPolicy := old.(*resource.AccessControlPolicy)

	newPolicy.Status = oldPolicy.Status
	newPolicy.Generation = oldPolicy.Generation
	if !apiequality.Semantic.DeepEqual(oldPolicy.Spec, newPolicy.Spec) {
		newPolicy.Generation = oldPolicy.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateAccessControlPolicy(obj.(*resource.AccessControlPolicy))
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
	newPolicy := obj.(*resource.AccessControlPolicy)
	oldPolicy := old.(*resource.AccessControlPolicy)

	newPolicy.Spec = oldPolicy.Spec
	metav1.ResetObjectMetaForStatus(&newPolicy.ObjectMeta, &oldPolicy.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateAccessControlPolicy(policy *resource.AccessControlPolicy) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}
	if policy.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	errs = append(errs, policytarget.ValidateRefs(policy.Spec.TargetRefs, specPath.Child("targetRefs"))...)
	switch policy.Spec.DefaultAction {
	case "", resource.AccessControlActionAllow, resource.AccessControlActionDeny:
	default:
		errs = append(errs, field.NotSupported(specPath.Child("defaultAction"), policy.Spec.DefaultAction, []string{
			string(resource.AccessControlActionAllow),
			string(resource.AccessControlActionDeny),
		}))
	}
	if len(policy.Spec.Rules) == 0 && policy.Spec.DefaultAction != resource.AccessControlActionDeny {
		errs = append(errs, field.Required(specPath.Child("rules"), "at least one rule is required unless defaultAction is Deny"))
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
		switch rule.Action {
		case resource.AccessControlActionAllow, resource.AccessControlActionDeny:
		default:
			errs = append(errs, field.NotSupported(rulePath.Child("action"), rule.Action, []string{
				string(resource.AccessControlActionAllow),
				string(resource.AccessControlActionDeny),
			}))
		}
		for j, condition := range rule.Conditions {
			errs = append(errs, validateAccessControlCondition(condition, rulePath.Child("conditions").Index(j))...)
		}
	}

	statusCode := policy.Spec.Response.StatusCode
	if statusCode != 0 && (statusCode < minResponseStatusCode || statusCode > maxResponseStatusCode) {
		errs = append(errs, field.Invalid(specPath.Child("response").Child("statusCode"), statusCode, "statusCode must be between 400 and 599"))
	}
	return errs
}

func validateAccessControlCondition(condition resource.AccessControlCondition, path *field.Path) field.ErrorList {
	errs := field.ErrorList{}
	if condition.Value == "" {
		errs = append(errs, field.Required(path.Child("value"), "condition value is required"))
	}
	switch condition.Type {
	case resource.AccessControlConditionTypeIP:
		if condition.Name != "" {
			errs = append(errs, field.Invalid(path.Child("name"), condition.Name, "name must be empty for IP conditions"))
		}
		if condition.Value != "" && !validIPOrPrefix(condition.Value) {
			errs = append(errs, field.Invalid(path.Child("value"), condition.Value, "value must be an IP address or CIDR prefix"))
		}
	case resource.AccessControlConditionTypeHeader:
		if condition.Name == "" {
			errs = append(errs, field.Required(path.Child("name"), "name is required for Header conditions"))
		} else if messages := k8svalidation.IsHTTPHeaderName(condition.Name); len(messages) > 0 {
			errs = append(errs, field.Invalid(path.Child("name"), condition.Name, messages[0]))
		}
	default:
		errs = append(errs, field.NotSupported(path.Child("type"), condition.Type, []string{
			string(resource.AccessControlConditionTypeIP),
			string(resource.AccessControlConditionTypeHeader),
		}))
	}
	return errs
}

func validIPOrPrefix(value string) bool {
	if _, err := netip.ParseAddr(value); err == nil {
		return true
	}
	_, err := netip.ParsePrefix(value)
	return err == nil
}
