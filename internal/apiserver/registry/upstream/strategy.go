package upstream

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	metav1.ResetObjectMetaForStatus(&newUpstream.ObjectMeta, &oldUpstream.ObjectMeta)
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
	if upstream.Spec.Type != "" && !validUpstreamType(upstream.Spec.Type) {
		errs = append(errs, field.NotSupported(specPath.Child("type"), upstream.Spec.Type, []string{
			string(resource.UpstreamTypeApplication),
			string(resource.UpstreamTypeModel),
			string(resource.UpstreamTypeAgent),
			string(resource.UpstreamTypeMCP),
		}))
	}
	if upstream.Spec.LoadBalancePolicy != "" && !validLoadBalancePolicy(upstream.Spec.LoadBalancePolicy) {
		errs = append(errs, field.NotSupported(specPath.Child("loadBalancePolicy"), upstream.Spec.LoadBalancePolicy, []string{
			string(resource.UpstreamLoadBalancePolicyRoundRobin),
			string(resource.UpstreamLoadBalancePolicyLeastRequest),
			string(resource.UpstreamLoadBalancePolicyRandom),
		}))
	}
	if upstream.Spec.HealthCheck != nil {
		healthCheckPath := specPath.Child("healthCheck")
		if upstream.Spec.HealthCheck.Enabled {
			if upstream.Spec.HealthCheck.Path == "" {
				errs = append(errs, field.Required(healthCheckPath.Child("path"), "path is required when health check is enabled"))
			}
			if upstream.Spec.HealthCheck.IntervalSeconds < 1 || upstream.Spec.HealthCheck.IntervalSeconds > 300 {
				errs = append(errs, field.Invalid(healthCheckPath.Child("intervalSeconds"), upstream.Spec.HealthCheck.IntervalSeconds, "intervalSeconds must be between 1 and 300"))
			}
			if upstream.Spec.HealthCheck.TimeoutSeconds < 1 || upstream.Spec.HealthCheck.TimeoutSeconds > 60 || upstream.Spec.HealthCheck.TimeoutSeconds >= upstream.Spec.HealthCheck.IntervalSeconds {
				errs = append(errs, field.Invalid(healthCheckPath.Child("timeoutSeconds"), upstream.Spec.HealthCheck.TimeoutSeconds, "timeoutSeconds must be between 1 and 60 and less than intervalSeconds"))
			}
		}
	}

	enabledEndpoints := 0
	endpointNames := map[string]struct{}{}
	for i, endpoint := range upstream.Spec.Endpoints {
		endpointPath := specPath.Child("endpoints").Index(i)
		if endpoint.Name != "" {
			if _, ok := endpointNames[endpoint.Name]; ok {
				errs = append(errs, field.Duplicate(endpointPath.Child("name"), endpoint.Name))
			}
			endpointNames[endpoint.Name] = struct{}{}
		}
		if endpoint.Address == "" {
			errs = append(errs, field.Required(endpointPath.Child("address"), "address is required"))
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 {
			errs = append(errs, field.Invalid(endpointPath.Child("port"), endpoint.Port, "port must be between 1 and 65535"))
		}
		if endpoint.Weight < 1 || endpoint.Weight > 100 {
			errs = append(errs, field.Invalid(endpointPath.Child("weight"), endpoint.Weight, "weight must be between 1 and 100"))
		}
		if endpoint.Enabled {
			enabledEndpoints++
		}
	}
	if len(upstream.Spec.Endpoints) > 0 && enabledEndpoints == 0 {
		errs = append(errs, field.Invalid(specPath.Child("endpoints"), upstream.Spec.Endpoints, "at least one endpoint must be enabled"))
	}

	return errs
}

func validUpstreamType(value resource.UpstreamType) bool {
	switch value {
	case resource.UpstreamTypeApplication, resource.UpstreamTypeModel, resource.UpstreamTypeAgent, resource.UpstreamTypeMCP:
		return true
	default:
		return false
	}
}

func validLoadBalancePolicy(value resource.UpstreamLoadBalancePolicy) bool {
	switch value {
	case resource.UpstreamLoadBalancePolicyRoundRobin, resource.UpstreamLoadBalancePolicyLeastRequest, resource.UpstreamLoadBalancePolicyRandom:
		return true
	default:
		return false
	}
}
