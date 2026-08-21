package route

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
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
)

// strategy 定义 Route 资源在 apiserver 存储前后的处理规则
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 Route status 子资源更新规则
type statusStrategy struct {
	strategy
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	route := obj.(*resource.Route)
	route.Status = resource.ResourceStatus{}
	canonicalizeRouteSpec(&route.Spec)
	apiregistry.PrepareObjectMetaForCreate(&route.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
}

func (strategy) Canonicalize(obj runtime.Object) {
	route := obj.(*resource.Route)
	canonicalizeRouteSpec(&route.Spec)
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Status = oldRoute.Status
	canonicalizeRouteSpec(&newRoute.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldRoute.Spec, newRoute.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newRoute.ObjectMeta, &oldRoute.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func (statusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return apiregistry.SpecResetFields()
}

func (statusStrategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Spec = oldRoute.Spec
	metav1.ResetObjectMetaForStatus(&newRoute.ObjectMeta, &oldRoute.ObjectMeta)
}

func (statusStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
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
	if spec.HostRewrite != nil {
		spec.HostRewrite.Hostname = strings.ToLower(strings.TrimSpace(spec.HostRewrite.Hostname))
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
