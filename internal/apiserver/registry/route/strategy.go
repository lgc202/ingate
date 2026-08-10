package route

import (
	"context"
	"maps"
	"strings"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

// strategy 定义 Route 资源在 apiserver 存储前后的处理规则
type strategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// statusStrategy 定义 Route status 子资源更新规则
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
	route := obj.(*resource.Route)
	route.Status = resource.ResourceStatus{}
	route.Generation = 1
	canonicalizeRouteSpec(&route.Spec)
	setUpdatedAt(&route.ObjectMeta, route.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	route := obj.(*resource.Route)
	canonicalizeRouteSpec(&route.Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Status = oldRoute.Status
	canonicalizeRouteSpec(&newRoute.Spec)
	newRoute.Generation = oldRoute.Generation
	if !apiequality.Semantic.DeepEqual(oldRoute.Spec, newRoute.Spec) {
		newRoute.Generation = oldRoute.Generation + 1
		setUpdatedAt(&newRoute.ObjectMeta, time.Now().UTC())
		return
	}
	preserveUpdatedAt(&newRoute.ObjectMeta, &oldRoute.ObjectMeta)
}

func (strategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
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
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Spec = oldRoute.Spec
	metav1.ResetObjectMetaForStatus(&newRoute.ObjectMeta, &oldRoute.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeRouteSpec(spec *resource.RouteSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.GatewayRefs {
		spec.GatewayRefs[i] = strings.TrimSpace(spec.GatewayRefs[i])
	}
	for i := range spec.Hostnames {
		hostname, ok := hostnameutil.Normalize(spec.Hostnames[i])
		if ok {
			spec.Hostnames[i] = hostname
		}
	}
	spec.Match.Path.Value = strings.TrimSpace(spec.Match.Path.Value)
	for i := range spec.Match.Methods {
		spec.Match.Methods[i] = strings.ToUpper(strings.TrimSpace(spec.Match.Methods[i]))
	}
	for i := range spec.Match.Headers {
		spec.Match.Headers[i].Name = strings.ToLower(strings.TrimSpace(spec.Match.Headers[i].Name))
	}
	for i := range spec.UpstreamRefs {
		spec.UpstreamRefs[i].Name = strings.TrimSpace(spec.UpstreamRefs[i].Name)
	}
	canonicalizeHeaderModifier(spec.RequestHeaderModifier)
	canonicalizeHeaderModifier(spec.ResponseHeaderModifier)
}

func canonicalizeHeaderModifier(modifier *resource.HeaderModifier) {
	if modifier == nil {
		return
	}
	for i := range modifier.Set {
		modifier.Set[i].Name = strings.ToLower(strings.TrimSpace(modifier.Set[i].Name))
	}
	for i := range modifier.Add {
		modifier.Add[i].Name = strings.ToLower(strings.TrimSpace(modifier.Add[i].Name))
	}
	for i := range modifier.Remove {
		modifier.Remove[i] = strings.ToLower(strings.TrimSpace(modifier.Remove[i]))
	}
}

func setUpdatedAt(metadata *metav1.ObjectMeta, updatedAt time.Time) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt.Format(time.RFC3339Nano)
}

func preserveUpdatedAt(metadata, oldMetadata *metav1.ObjectMeta) {
	metadata.Annotations = maps.Clone(metadata.Annotations)
	delete(metadata.Annotations, resource.AnnotationUpdatedAt)
	updatedAt := oldMetadata.Annotations[resource.AnnotationUpdatedAt]
	if updatedAt == "" {
		return
	}
	if metadata.Annotations == nil {
		metadata.Annotations = make(map[string]string, 1)
	}
	metadata.Annotations[resource.AnnotationUpdatedAt] = updatedAt
}
