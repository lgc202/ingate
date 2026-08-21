package upstream

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

const (
	defaultEndpointWeight             = 1
	defaultHealthCheckIntervalSeconds = 10
	defaultHealthCheckTimeoutSeconds  = 2
)

// strategy 定义 Upstream 资源在 apiserver 存储前后的处理规则
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 Upstream status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	upstream := obj.(*resource.Upstream)
	upstream.Status = resource.ResourceStatus{}
	canonicalizeUpstreamSpec(&upstream.Spec)
	apiregistry.PrepareObjectMetaForCreate(&upstream.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateUpstream(obj.(*resource.Upstream))
}

func (strategy) Canonicalize(obj runtime.Object) {
	upstream := obj.(*resource.Upstream)
	canonicalizeUpstreamSpec(&upstream.Spec)
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newUpstream := obj.(*resource.Upstream)
	oldUpstream := old.(*resource.Upstream)

	newUpstream.Status = oldUpstream.Status
	canonicalizeUpstreamSpec(&newUpstream.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldUpstream.Spec, newUpstream.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newUpstream.ObjectMeta, &oldUpstream.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateUpstream(obj.(*resource.Upstream))
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newUpstream := obj.(*resource.Upstream)
	oldUpstream := old.(*resource.Upstream)

	newUpstream.Spec = oldUpstream.Spec
	metav1.ResetObjectMetaForStatus(&newUpstream.ObjectMeta, &oldUpstream.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
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
