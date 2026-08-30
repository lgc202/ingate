package wasmplugin

import (
	"context"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/wasmconfig"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	plugin := obj.(*resource.WasmPlugin)
	plugin.Status = resource.ResourceStatus{}
	canonicalizeSpec(&plugin.Spec)
	apiregistry.PrepareObjectMetaForCreate(&plugin.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validatePlugin(obj.(*resource.WasmPlugin))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPlugin := obj.(*resource.WasmPlugin)
	oldPlugin := old.(*resource.WasmPlugin)

	newPlugin.Status = oldPlugin.Status
	canonicalizeSpec(&newPlugin.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldPlugin.Spec, newPlugin.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newPlugin.ObjectMeta, &oldPlugin.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, old runtime.Object) field.ErrorList {
	plugin := obj.(*resource.WasmPlugin)
	oldPlugin := old.(*resource.WasmPlugin)
	errList := validatePlugin(plugin)
	specPath := field.NewPath("spec")
	errList = append(errList, apivalidation.ValidateImmutableField(
		plugin.Spec.SourceID,
		oldPlugin.Spec.SourceID,
		specPath.Child("sourceID"),
	)...)
	errList = append(errList, apivalidation.ValidateImmutableField(
		plugin.Spec.Package,
		oldPlugin.Spec.Package,
		specPath.Child("package"),
	)...)
	return errList
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newPlugin := obj.(*resource.WasmPlugin)
	oldPlugin := old.(*resource.WasmPlugin)

	newPlugin.Spec = oldPlugin.Spec
	metav1.ResetObjectMetaForStatus(&newPlugin.ObjectMeta, &oldPlugin.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	plugin := obj.(*resource.WasmPlugin)
	return apiregistry.ValidateResourceStatus(plugin.Status, plugin.Generation)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func canonicalizeSpec(spec *resource.WasmPluginSpec) {
	if sourceID, valid := resourceconfig.NormalizeID(spec.SourceID); valid {
		spec.SourceID = sourceID
	}
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.Package = strings.ToLower(strings.TrimSpace(spec.Package))
	spec.Version = wasmconfig.NormalizeVersion(spec.Version)
	spec.URL = strings.TrimSpace(spec.URL)
	spec.SHA256 = strings.TrimSpace(spec.SHA256)
	spec.RootID = strings.TrimSpace(spec.RootID)
	if spec.PullPolicy == "" {
		spec.PullPolicy = resource.WasmPluginPullIfNotPresent
	}
}
