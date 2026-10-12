package validation

import (
	"net/url"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/lgc202/ingate/pkg/apis/gateway"
)

// ValidateAIBackend 校验新建 AIBackend 的身份及单对象访问契约，不解析 Backend 引用。
func ValidateAIBackend(backend *gateway.AIBackend) field.ErrorList {
	errs := apivalidation.ValidateObjectMeta(
		&backend.ObjectMeta,
		false,
		apivalidation.NameIsDNSSubdomain,
		field.NewPath("metadata"),
	)

	return append(errs, validateAIBackendSpec(&backend.Spec, field.NewPath("spec"))...)
}

// ValidateAIBackendUpdate 校验 AIBackend 的资源更新及元数据并发前提。
func ValidateAIBackendUpdate(next, previous *gateway.AIBackend) field.ErrorList {
	errs := ValidateAIBackend(next)
	return append(errs, apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)...)
}

// ValidateAIBackendStatusUpdate 校验观测状态，不允许 observedGeneration 超过资源代数。
func ValidateAIBackendStatusUpdate(next, previous *gateway.AIBackend) field.ErrorList {
	errs := apivalidation.ValidateObjectMetaUpdate(
		&next.ObjectMeta,
		&previous.ObjectMeta,
		field.NewPath("metadata"),
	)

	return append(errs, validateConditions(next.Status.Conditions, next.Generation)...)
}

func validateAIBackendSpec(spec *gateway.AIBackendSpec, path *field.Path) field.ErrorList {
	errs := validateReferenceName(spec.BackendRef.Name, path.Child("backendRef", "name"))
	schemaPath := path.Child("schema")
	if spec.Schema.Name == "" {
		errs = append(errs, field.Required(schemaPath.Child("name"), "an API schema is required"))
	} else if spec.Schema.Name != gateway.APISchemaOpenAI {
		errs = append(errs, field.NotSupported(
			schemaPath.Child("name"),
			spec.Schema.Name,
			[]gateway.APISchemaName{gateway.APISchemaOpenAI},
		))
	}

	return append(errs, validateAPISchemaPrefix(spec.Schema.Prefix, schemaPath.Child("prefix"))...)
}

func validateAPISchemaPrefix(prefix string, path *field.Path) field.ErrorList {
	parsed, err := url.Parse(prefix)
	if err != nil {
		return field.ErrorList{field.Invalid(path, prefix, "must be a valid API path prefix")}
	}
	if parsed.IsAbs() || parsed.Host != "" || parsed.User != nil || strings.ContainsAny(prefix, "?#") {
		return field.ErrorList{field.Invalid(path, prefix, "must be a path prefix without an authority, query or fragment")}
	}

	return nil
}
