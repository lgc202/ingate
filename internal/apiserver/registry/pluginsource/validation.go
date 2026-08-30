package pluginsource

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/httpurl"
)

func validateSource(source *resource.PluginSource) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := apiregistry.ValidateResourceID(source.Name, field.NewPath("metadata", "name"))

	errs = append(errs, apiregistry.ValidateDisplayName(
		source.Spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	if !httpurl.IsValid(source.Spec.URL) {
		errs = append(errs, field.Invalid(
			specPath.Child("url"),
			source.Spec.URL,
			"must be a valid HTTP or HTTPS catalog URL",
		))
	}
	return errs
}
