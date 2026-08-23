package pluginsource

import (
	"errors"
	"net/url"

	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

var errInvalidCatalogURL = errors.New("url must identify an HTTP or HTTPS plugin catalog")

func validateSource(source *resource.PluginSource) field.ErrorList {
	specPath := field.NewPath("spec")
	var errs field.ErrorList

	if source.Spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	parsed, err := url.ParseRequestURI(source.Spec.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		errs = append(errs, field.Invalid(specPath.Child("url"), source.Spec.URL, errInvalidCatalogURL.Error()))
	}
	return errs
}
