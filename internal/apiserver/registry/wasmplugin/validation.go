package wasmplugin

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

var (
	sha256Pattern                 = regexp.MustCompile(`^[a-f0-9]{64}$`)
	errInvalidPluginURL           = errors.New("url must identify a remote Wasm module or OCI image")
	errUnsupportedPluginURLScheme = errors.New("url scheme must be http, https, or oci")
)

func validatePlugin(plugin *resource.WasmPlugin) field.ErrorList {
	specPath := field.NewPath("spec")
	spec := plugin.Spec
	var errs field.ErrorList

	if spec.SourceID == "" {
		errs = append(errs, field.Required(specPath.Child("sourceID"), "sourceID is required"))
	}
	if spec.DisplayName == "" {
		errs = append(errs, field.Required(specPath.Child("displayName"), "displayName is required"))
	}
	if messages := utilvalidation.IsDNS1123Subdomain(spec.Package); len(messages) > 0 {
		errs = append(errs, field.Invalid(specPath.Child("package"), spec.Package, strings.Join(messages, "; ")))
	} else if !resource.IsSupportedWasmPluginPackage(spec.Package) {
		errs = append(errs, field.NotSupported(
			specPath.Child("package"),
			spec.Package,
			resource.SupportedWasmPluginPackages(),
		))
	}
	if spec.Version == "" {
		errs = append(errs, field.Required(specPath.Child("version"), "version is required"))
	}
	if err := validateURL(spec.URL); err != nil {
		errs = append(errs, field.Invalid(specPath.Child("url"), spec.URL, err.Error()))
	}
	if spec.SHA256 != "" && !sha256Pattern.MatchString(spec.SHA256) {
		errs = append(errs, field.Invalid(specPath.Child("sha256"), spec.SHA256, "sha256 must contain 64 lowercase hexadecimal characters"))
	}
	switch spec.PullPolicy {
	case resource.WasmPluginPullIfNotPresent, resource.WasmPluginPullAlways:
	default:
		errs = append(errs, field.NotSupported(specPath.Child("pullPolicy"), spec.PullPolicy, []string{
			string(resource.WasmPluginPullIfNotPresent),
			string(resource.WasmPluginPullAlways),
		}))
	}
	return errs
}

func validateURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Host == "" {
		return &url.Error{Op: "parse", URL: value, Err: errInvalidPluginURL}
	}
	switch parsed.Scheme {
	case "http", "https":
		return nil
	case "oci":
		if strings.Trim(parsed.Path, "/") == "" {
			return &url.Error{Op: "parse", URL: value, Err: errInvalidPluginURL}
		}
		return nil
	default:
		return &url.Error{Op: "parse", URL: value, Err: errUnsupportedPluginURLScheme}
	}
}
