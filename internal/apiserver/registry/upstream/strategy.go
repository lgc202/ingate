package upstream

import (
	"cmp"
	"context"
	"slices"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"

	apiregistry "github.com/lgc202/ingate/internal/apiserver/registry"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway"
	"github.com/lgc202/ingate/internal/pkg/upstreamconfig"
)

// strategy 定义 Upstream 资源在 API Server 存储前后的处理规则。
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 Upstream status 子资源更新规则。
type statusStrategy struct {
	strategy
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

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newUpstream := obj.(*resource.Upstream)
	oldUpstream := old.(*resource.Upstream)

	newUpstream.Spec = oldUpstream.Spec
	metav1.ResetObjectMetaForStatus(&newUpstream.ObjectMeta, &oldUpstream.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	upstream := obj.(*resource.Upstream)
	return apiregistry.ValidateResourceStatus(upstream.Status, upstream.Generation)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func canonicalizeUpstreamSpec(spec *resource.UpstreamSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	if spec.LoadBalancing == "" {
		spec.LoadBalancing = resource.LoadBalancingRoundRobin
	}
	for i := range spec.Endpoints {
		spec.Endpoints[i].Address = upstreamconfig.NormalizeAddress(spec.Endpoints[i].Address)
		if spec.Endpoints[i].Weight == 0 {
			spec.Endpoints[i].Weight = upstreamconfig.DefaultEndpointWeight
		}
	}
	slices.SortFunc(spec.Endpoints, func(left, right resource.Endpoint) int {
		return cmp.Or(
			strings.Compare(left.Address, right.Address),
			cmp.Compare(left.Port, right.Port),
			cmp.Compare(left.Weight, right.Weight),
		)
	})
	if spec.TLS != nil {
		spec.TLS.ServerName = upstreamconfig.NormalizeAddress(spec.TLS.ServerName)
	}
	if spec.HealthCheck != nil {
		spec.HealthCheck.Path = strings.TrimSpace(spec.HealthCheck.Path)
		if spec.HealthCheck.IntervalSeconds == 0 {
			spec.HealthCheck.IntervalSeconds = upstreamconfig.DefaultHealthCheckIntervalSeconds
		}
		if spec.HealthCheck.TimeoutSeconds == 0 {
			spec.HealthCheck.TimeoutSeconds = upstreamconfig.DefaultHealthCheckTimeoutSeconds
		}
	}
}
