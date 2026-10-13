package validation

import (
	"net/url"
	"strings"
	"unicode/utf8"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

// ValidateMCPRoute 校验资源身份、父引用和路径，不解析 Backend 引用。
func ValidateMCPRoute(route *gateway.MCPRoute) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&route.ObjectMeta,
		true,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)
	return append(errs, validateMCPRouteSpec(&route.Spec, field.NewPath("spec"))...)
}

// ValidateMCPRouteUpdate 校验更新后的 MCPRoute 及元数据并发前提。
func ValidateMCPRouteUpdate(next, previous *gateway.MCPRoute) field.ErrorList {
	errs := ValidateMCPRoute(next)
	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateMCPRouteStatusUpdate 校验父状态和观测版本，不接受重复的控制器父记录。
func ValidateMCPRouteStatusUpdate(next, previous *gateway.MCPRoute) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)
	return append(errs, validateRouteParentStatuses(next.Status.Parents, next.Generation)...)
}

func validateMCPRouteSpec(spec *gateway.MCPRouteSpec, path *field.Path) field.ErrorList {
	parentsPath := path.Child("parentRefs")
	errs := validateParentRefs(spec.ParentRefs, parentsPath)
	if len(spec.ParentRefs) > 16 {
		errs = append(errs, field.TooMany(parentsPath, len(spec.ParentRefs), 16))
	}
	errs = append(errs, validateMCPPath(spec.Path, path.Child("path"))...)

	refsPath := path.Child("backendRefs")
	if len(spec.BackendRefs) != 1 {
		errs = append(errs, field.Invalid(refsPath, len(spec.BackendRefs), "exactly one Backend reference is supported"))
	}
	for i, ref := range spec.BackendRefs {
		refPath := refsPath.Index(i)
		errs = append(errs, validateReferenceName(ref.Name, refPath.Child("name"))...)
		errs = append(errs, validateMCPPath(ref.Path, refPath.Child("path"))...)
	}

	return errs
}

func validateMCPPath(value *string, path *field.Path) field.ErrorList {
	if value == nil {
		return nil
	}
	if *value == "" {
		return field.ErrorList{field.Required(path, "an absolute path is required")}
	}

	var errs field.ErrorList
	if utf8.RuneCountInString(*value) > 1024 {
		errs = append(errs, field.TooLongCharacters(path, *value, 1024))
	}
	parsed, err := url.Parse(*value)
	if err != nil {
		return append(errs, field.Invalid(path, *value, "must be a valid absolute path"))
	}
	if !strings.HasPrefix(*value, "/") || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || strings.ContainsAny(*value, "?#") {
		errs = append(errs, field.Invalid(path, *value, "must be an absolute path without authority, query or fragment"))
	}

	return errs
}
