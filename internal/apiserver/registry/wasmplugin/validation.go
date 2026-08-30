package wasmplugin

import (
	"strings"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/wasmconfig"
)

func validatePlugin(plugin *resource.WasmPlugin) field.ErrorList {
	metadataNamePath := field.NewPath("metadata", "name")
	specPath := field.NewPath("spec")
	spec := plugin.Spec
	errs := apiregistry.ValidateResourceID(plugin.Name, metadataNamePath)

	if spec.SourceID == "" {
		errs = append(errs, field.Required(specPath.Child("sourceID"), "sourceID is required"))
	} else if !resourceconfig.IsCanonicalID(spec.SourceID) {
		errs = append(errs, field.Invalid(
			specPath.Child("sourceID"),
			spec.SourceID,
			"sourceID must be a canonical UUID",
		))
	}
	errs = append(errs, apiregistry.ValidateDisplayName(
		spec.DisplayName,
		specPath.Child("displayName"),
	)...)
	packageValid := true
	if messages := utilvalidation.IsDNS1123Subdomain(spec.Package); len(messages) > 0 {
		errs = append(errs, field.Invalid(specPath.Child("package"), spec.Package, strings.Join(messages, "; ")))
		packageValid = false
	} else if !resource.IsSupportedWasmPluginPackage(spec.Package) {
		errs = append(errs, field.NotSupported(
			specPath.Child("package"),
			spec.Package,
			resource.SupportedWasmPluginPackages(),
		))
		packageValid = false
	}
	if packageValid && plugin.Name != wasmconfig.PluginID(spec.Package) {
		errs = append(errs, field.Invalid(
			metadataNamePath,
			plugin.Name,
			"must be the stable resource ID derived from spec.package",
		))
	}
	if !wasmconfig.IsValidVersion(spec.Version) {
		errs = append(errs, field.Invalid(
			specPath.Child("version"),
			spec.Version,
			"version must be a semantic version without a v prefix",
		))
	}
	if !wasmconfig.IsValidArtifactURL(spec.URL) {
		errs = append(errs, field.Invalid(
			specPath.Child("url"),
			spec.URL,
			"url must identify an HTTP, HTTPS, or OCI Wasm artifact",
		))
	}
	if spec.SHA256 == "" {
		errs = append(errs, field.Required(specPath.Child("sha256"), "sha256 is required"))
	} else if !wasmconfig.IsValidSHA256Digest(spec.SHA256) {
		errs = append(errs, field.Invalid(
			specPath.Child("sha256"),
			spec.SHA256,
			"sha256 must contain 64 lowercase hexadecimal characters",
		))
	}
	if !wasmconfig.IsValidRootID(spec.RootID) {
		errs = append(errs, field.Invalid(
			specPath.Child("rootID"),
			spec.RootID,
			"rootID must not exceed 256 bytes or contain control characters",
		))
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
