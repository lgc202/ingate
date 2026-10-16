package validation

import (
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

// ValidateModelRoute 校验资源身份、父引用和模型规则，不解析 AIBackend 引用。
func ValidateModelRoute(route *gateway.ModelRoute) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&route.ObjectMeta,
		true,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)

	return append(errs, validateModelRouteSpec(&route.Spec, field.NewPath("spec"))...)
}

// ValidateModelRouteUpdate 校验更新后的 ModelRoute 及元数据并发前提。
func ValidateModelRouteUpdate(next, previous *gateway.ModelRoute) field.ErrorList {
	errs := ValidateModelRoute(next)

	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateModelRouteStatusUpdate 校验父状态和观测版本，不接受重复的控制器父记录。
func ValidateModelRouteStatusUpdate(next, previous *gateway.ModelRoute) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)

	return append(errs, validateRouteParentStatuses(next.Status.Parents, next.Generation)...)
}

func validateModelRouteSpec(spec *gateway.ModelRouteSpec, path *field.Path) field.ErrorList {
	parentsPath := path.Child("parentRefs")
	errs := validateParentRefs(spec.ParentRefs, parentsPath)
	if len(spec.ParentRefs) > 16 {
		errs = append(errs, field.TooMany(parentsPath, len(spec.ParentRefs), 16))
	}

	rulesPath := path.Child("rules")
	if len(spec.Rules) == 0 {
		errs = append(errs, field.Required(rulesPath, "at least one rule is required"))
	}

	if len(spec.Rules) > 15 {
		errs = append(errs, field.TooMany(rulesPath, len(spec.Rules), 15))
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

		errs = append(errs, validateModelRouteRule(&rule, rulePath)...)
	}

	return errs
}

func validateModelRouteRule(rule *gateway.ModelRouteRule, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	refsPath := path.Child("backendRefs")
	if len(rule.BackendRefs) != 1 {
		errs = append(errs, field.Invalid(refsPath, len(rule.BackendRefs), "exactly one AIBackend reference is supported"))
	}

	for i, ref := range rule.BackendRefs {
		errs = append(errs, validateReferenceName(ref.Name, refsPath.Index(i).Child("name"))...)
	}

	matchesPath := path.Child("matches")
	if len(rule.Matches) > 128 {
		errs = append(errs, field.TooMany(matchesPath, len(rule.Matches), 128))
	}

	for i, match := range rule.Matches {
		headersPath := matchesPath.Index(i).Child("headers")
		if len(match.Headers) > 1 {
			errs = append(errs, field.TooMany(headersPath, len(match.Headers), 1))
		}

		for j, header := range match.Headers {
			errs = append(errs, validateModelHeaderMatch(header, headersPath.Index(j))...)
		}
	}

	return errs
}

func validateModelHeaderMatch(header gateway.HTTPHeaderMatch, path *field.Path) field.ErrorList {
	var errs field.ErrorList
	if header.Name == "" {
		errs = append(errs, field.Required(path.Child("name"), "a model header name is required"))
	} else if !strings.EqualFold(header.Name, "x-ai-eg-model") {
		errs = append(errs, field.NotSupported(path.Child("name"), header.Name, []string{"x-ai-eg-model"}))
	}

	if header.Type != "" && header.Type != gateway.HeaderMatchExact {
		errs = append(errs, field.NotSupported(path.Child("type"), header.Type, []gateway.HeaderMatchType{gateway.HeaderMatchExact}))
	}

	if header.Value == "" {
		errs = append(errs, field.Required(path.Child("value"), "a model name is required"))
	}

	return errs
}
