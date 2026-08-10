package upstream

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

	resource "github.com/lgc202/ingate/pkg/apis/gateway"
)

const gatewayAPIVersion = "gateway.ingate.io/v1"

const (
	defaultEndpointWeight             = 1
	defaultHealthCheckIntervalSeconds = 10
	defaultHealthCheckTimeoutSeconds  = 2
)

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
	canonicalizeUpstreamSpec(&upstream.Spec)
	setUpdatedAt(&upstream.ObjectMeta, upstream.CreationTimestamp.Time)
}

func (strategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return validateUpstream(obj.(*resource.Upstream))
}

func (strategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (strategy) Canonicalize(obj runtime.Object) {
	upstream := obj.(*resource.Upstream)
	canonicalizeUpstreamSpec(&upstream.Spec)
}

func (strategy) AllowCreateOnUpdate() bool {
	return false
}

func (strategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newUpstream := obj.(*resource.Upstream)
	oldUpstream := old.(*resource.Upstream)

	newUpstream.Status = oldUpstream.Status
	canonicalizeUpstreamSpec(&newUpstream.Spec)
	newUpstream.Generation = oldUpstream.Generation
	if !apiequality.Semantic.DeepEqual(oldUpstream.Spec, newUpstream.Spec) {
		newUpstream.Generation = oldUpstream.Generation + 1
		setUpdatedAt(&newUpstream.ObjectMeta, time.Now().UTC())
		return
	}
	preserveUpdatedAt(&newUpstream.ObjectMeta, &oldUpstream.ObjectMeta)
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
	metav1.ResetObjectMetaForStatus(&newUpstream.ObjectMeta, &oldUpstream.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return nil
}

func canonicalizeUpstreamSpec(spec *resource.UpstreamSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	if spec.LoadBalancing == "" {
		spec.LoadBalancing = resource.LoadBalancingRoundRobin
	}
	for i := range spec.Endpoints {
		spec.Endpoints[i].Address = strings.ToLower(strings.TrimSpace(spec.Endpoints[i].Address))
		if spec.Endpoints[i].Weight == 0 {
			spec.Endpoints[i].Weight = defaultEndpointWeight
		}
	}
	if spec.TLS != nil {
		spec.TLS.ServerName = strings.ToLower(strings.TrimSpace(spec.TLS.ServerName))
	}
	if spec.HealthCheck != nil {
		spec.HealthCheck.Path = strings.TrimSpace(spec.HealthCheck.Path)
		if spec.HealthCheck.IntervalSeconds == 0 {
			spec.HealthCheck.IntervalSeconds = defaultHealthCheckIntervalSeconds
		}
		if spec.HealthCheck.TimeoutSeconds == 0 {
			spec.HealthCheck.TimeoutSeconds = defaultHealthCheckTimeoutSeconds
		}
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
