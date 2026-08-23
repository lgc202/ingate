package pluginsource

import (
	"context"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
)

type strategy struct {
	apiregistry.Strategy
}

type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	source := obj.(*resource.PluginSource)
	source.Status = resource.ResourceStatus{}
	canonicalizeSpec(&source.Spec)
	apiregistry.PrepareObjectMetaForCreate(&source.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateSource(obj.(*resource.PluginSource))
}

func (strategy) Canonicalize(obj runtime.Object) {
	canonicalizeSpec(&obj.(*resource.PluginSource).Spec)
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newSource := obj.(*resource.PluginSource)
	oldSource := old.(*resource.PluginSource)

	newSource.Status = oldSource.Status
	canonicalizeSpec(&newSource.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldSource.Spec, newSource.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newSource.ObjectMeta, &oldSource.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateSource(obj.(*resource.PluginSource))
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newSource := obj.(*resource.PluginSource)
	oldSource := old.(*resource.PluginSource)

	newSource.Spec = oldSource.Spec
	metav1.ResetObjectMetaForStatus(&newSource.ObjectMeta, &oldSource.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeSpec(spec *resource.PluginSourceSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	spec.URL = strings.TrimSpace(spec.URL)
}
