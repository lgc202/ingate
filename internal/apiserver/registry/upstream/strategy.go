package upstream

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 Upstream 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 Upstream status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{
		ObjectTyper:   typer,
		NameGenerator: names.SimpleNameGenerator,
	}
}

func (strategy) NamespaceScoped() bool {
	return false
}

func (strategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}
}

func (strategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	upstream := obj.(*resource.Upstream)
	upstream.Status = resource.ResourceStatus{}
	upstream.Generation = 1
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateUpstream(obj.(*resource.Upstream))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newUpstream := obj.(*resource.Upstream)
	oldUpstream := old.(*resource.Upstream)

	newUpstream.Status = oldUpstream.Status
	newUpstream.Generation = oldUpstream.Generation
	if !apiequality.Semantic.DeepEqual(oldUpstream.Spec, newUpstream.Spec) {
		newUpstream.Generation = oldUpstream.Generation + 1
	}
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateUpstream(obj.(*resource.Upstream))
}

func (strategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (strategy) AllowUnconditionalUpdate() bool {
	return false
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		fieldpath.APIVersion(gatewayAPIVersion): fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (statusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newUpstream := obj.(*resource.Upstream)
	oldUpstream := old.(*resource.Upstream)

	newUpstream.Spec = oldUpstream.Spec
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func validateUpstream(upstream *resource.Upstream) field.ErrorList {
	specPath := field.NewPath("spec")
	errs := field.ErrorList{}

	if len(upstream.Spec.Endpoints) == 0 {
		errs = append(errs, field.Required(specPath.Child("endpoints"), "at least one endpoint is required"))
	}
	for i, endpoint := range upstream.Spec.Endpoints {
		endpointPath := specPath.Child("endpoints").Index(i)
		if endpoint.Address == "" {
			errs = append(errs, field.Required(endpointPath.Child("address"), "address is required"))
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 {
			errs = append(errs, field.Invalid(endpointPath.Child("port"), endpoint.Port, "port must be between 1 and 65535"))
		}
	}

	return errs
}
