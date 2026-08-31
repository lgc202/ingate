package registry

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
)

// ValidateResourceID 校验 metadata.name 使用的声明式资源 ID。
func ValidateResourceID(value string, path *field.Path) field.ErrorList {
	if resourceconfig.IsCanonicalID(value) {
		return nil
	}
	return field.ErrorList{field.Invalid(path, value, "resource ID must be a canonical UUID")}
}

// ValidateDisplayName 校验声明式资源共享的用户展示名称。
func ValidateDisplayName(value string, path *field.Path) field.ErrorList {
	switch {
	case value == "":
		return field.ErrorList{field.Required(path, "displayName is required")}
	case len(value) > resourceconfig.MaxDisplayNameBytes:
		return field.ErrorList{field.TooLong(path, value, resourceconfig.MaxDisplayNameBytes)}
	case !resourceconfig.IsValidDisplayName(value):
		return field.ErrorList{field.Invalid(
			path,
			value,
			"displayName must be valid UTF-8 without control characters",
		)}
	default:
		return nil
	}
}
