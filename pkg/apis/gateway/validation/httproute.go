package validation

import (
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

// ValidateHTTPRoute 校验资源身份、父引用和当前支持的 HTTP 规则。
func ValidateHTTPRoute(route *gateway.HTTPRoute) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&route.ObjectMeta,
		false,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)
	return append(errs, validateHTTPRouteSpec(&route.Spec, field.NewPath("spec"))...)
}

// ValidateHTTPRouteUpdate 校验更新后的 HTTPRoute。
func ValidateHTTPRouteUpdate(next, previous *gateway.HTTPRoute) field.ErrorList {
	errs := ValidateHTTPRoute(next)
	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateHTTPRouteStatusUpdate 校验父状态和观测版本，不接受重复的控制器父记录。
func ValidateHTTPRouteStatusUpdate(next, previous *gateway.HTTPRoute) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)

	seen := make(map[string]bool)
	for i, parent := range next.Status.Parents {
		path := field.NewPath("status", "parents").Index(i)
		errs = append(errs, validateReferenceName(parent.ParentRef.Name, path.Child("parentRef", "name"))...)
		if parent.ParentRef.SectionName != "" {
			errs = append(errs, validateListenerName(parent.ParentRef.SectionName, path.Child("parentRef", "sectionName"))...)
		}
		if parent.ControllerName == "" {
			errs = append(errs, field.Required(path.Child("controllerName"), "is required"))
		} else {
			errs = append(errs, validation.IsDomainPrefixedPath(path.Child("controllerName"), parent.ControllerName)...)
		}
		key := parent.ControllerName + "/" + parent.ParentRef.Name + "/" + parent.ParentRef.SectionName
		if seen[key] {
			errs = append(errs, field.Duplicate(path, key))
		}
		seen[key] = true
		errs = append(errs, metav1validation.ValidateConditions(parent.Conditions, path.Child("conditions"))...)
		for j, condition := range parent.Conditions {
			if condition.ObservedGeneration > next.Generation {
				errs = append(errs, field.Invalid(
					path.Child("conditions").Index(j).Child("observedGeneration"),
					condition.ObservedGeneration,
					"must not exceed metadata.generation",
				))
			}
		}
	}

	return errs
}

func validateHTTPRouteSpec(spec *gateway.HTTPRouteSpec, path *field.Path) field.ErrorList {
	errs := validateParentRefs(spec.ParentRefs, path.Child("parentRefs"))
	rulesPath := path.Child("rules")
	if len(spec.Rules) == 0 {
		errs = append(errs, field.Required(rulesPath, "at least one rule is required"))
	}

	names := make(map[string]bool)
	for i, rule := range spec.Rules {
		rulePath := rulesPath.Index(i)
		if rule.Name != "" {
			errs = append(errs, validateListenerName(rule.Name, rulePath.Child("name"))...)
			if names[rule.Name] {
				errs = append(errs, field.Duplicate(rulePath.Child("name"), rule.Name))
			}
			names[rule.Name] = true
		}

		if len(rule.BackendRefs) != 1 {
			errs = append(errs, field.Invalid(
				rulePath.Child("backendRefs"),
				len(rule.BackendRefs),
				"exactly one Backend reference is supported",
			))
		}
		for j, ref := range rule.BackendRefs {
			refPath := rulePath.Child("backendRefs").Index(j).Child("name")
			errs = append(errs, validateReferenceName(ref.Name, refPath)...)
		}

		for j, match := range rule.Matches {
			if match.Path != nil {
				matchPath := rulePath.Child("matches").Index(j).Child("path")
				errs = append(errs, validatePathMatch(*match.Path, matchPath)...)
			}
		}
	}

	return errs
}

func validateParentRefs(refs []gateway.ParentReference, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if len(refs) == 0 {
		errs = append(errs, field.Required(path, "at least one parent is required"))
	}

	seen := make(map[gateway.ParentReference]bool)
	whole := make(map[string]bool)
	for i, ref := range refs {
		refPath := path.Index(i)
		errs = append(errs, validateReferenceName(ref.Name, refPath.Child("name"))...)
		if ref.SectionName != "" {
			errs = append(errs, validateListenerName(ref.SectionName, refPath.Child("sectionName"))...)
		}
		if seen[ref] {
			errs = append(errs, field.Duplicate(refPath, ref))
		}
		seen[ref] = true
		if ref.SectionName == "" {
			whole[ref.Name] = true
		}
	}

	for i, ref := range refs {
		if ref.SectionName != "" && whole[ref.Name] {
			errs = append(errs, field.Invalid(
				path.Index(i),
				ref,
				"references to the same Gateway must select distinct explicit sections",
			))
		}
	}

	return errs
}

func validatePathMatch(match gateway.PathMatch, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if match.Type != "" && match.Type != gateway.PathMatchExact && match.Type != gateway.PathMatchPrefix {
		errs = append(errs, field.NotSupported(
			path.Child("type"),
			match.Type,
			[]string{string(gateway.PathMatchExact), string(gateway.PathMatchPrefix)},
		))
	}
	if match.Value != "" && (!strings.HasPrefix(match.Value, "/") ||
		strings.ContainsAny(match.Value, "?#") || strings.Contains(match.Value, "//")) {
		errs = append(errs, field.Invalid(
			path.Child("value"),
			match.Value,
			"must be an absolute path without consecutive slashes, query, or fragment",
		))
	}
	return errs
}

func validateReferenceName(name string, path *field.Path) field.ErrorList {
	if name == "" {
		return field.ErrorList{field.Required(path, "is required")}
	}
	if messages := apivalidation.NameIsDNSSubdomain(name, false); len(messages) > 0 {
		return field.ErrorList{field.Invalid(path, name, strings.Join(messages, "; "))}
	}
	return nil
}

func validateListenerName(name string, path *field.Path) field.ErrorList {
	if name == "" {
		return field.ErrorList{field.Required(path, "is required")}
	}
	if messages := apivalidation.NameIsDNSLabel(name, false); len(messages) > 0 {
		return field.ErrorList{field.Invalid(path, name, strings.Join(messages, "; "))}
	}
	return nil
}
