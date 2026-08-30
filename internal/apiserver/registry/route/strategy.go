package route

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
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/httpheader"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

// strategy 定义 Route 资源在 API Server 存储前后的处理规则。
type strategy struct {
	apiregistry.Strategy
}

// statusStrategy 定义 Route status 子资源更新规则。
type statusStrategy struct {
	strategy
}

func (strategy) PrepareForCreate(_ context.Context, obj runtime.Object) {
	route := obj.(*resource.Route)
	route.Status = resource.ResourceStatus{}
	defaultRouteSpec(&route.Spec)
	canonicalizeRouteSpec(&route.Spec)
	apiregistry.PrepareObjectMetaForCreate(&route.ObjectMeta)
}

func (strategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
}

func (strategy) PrepareForUpdate(_ context.Context, obj, old runtime.Object) {
	newRoute := obj.(*resource.Route)
	oldRoute := old.(*resource.Route)

	newRoute.Status = oldRoute.Status
	defaultRouteSpec(&newRoute.Spec)
	canonicalizeRouteSpec(&newRoute.Spec)
	specChanged := !apiequality.Semantic.DeepEqual(oldRoute.Spec, newRoute.Spec)
	apiregistry.PrepareObjectMetaForUpdate(&newRoute.ObjectMeta, &oldRoute.ObjectMeta, specChanged)
}

func (strategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateRoute(obj.(*resource.Route))
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

func (statusStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	route := obj.(*resource.Route)
	return apiregistry.ValidateResourceStatus(route.Status, route.Generation)
}

func newStrategy(typer runtime.ObjectTyper) strategy {
	return strategy{Strategy: apiregistry.NewStrategy(typer)}
}

func newStatusStrategy(typer runtime.ObjectTyper) statusStrategy {
	return statusStrategy{strategy: newStrategy(typer)}
}

func defaultRouteSpec(spec *resource.RouteSpec) {
	if spec.HostRewrite.Mode == "" {
		spec.HostRewrite.Mode = resource.HostRewriteUpstreamHost
	}
	if spec.Timeout.RequestMillis == 0 {
		spec.Timeout.RequestMillis = routeconfig.DefaultRequestTimeoutMillis
	}
}

func canonicalizeRouteSpec(spec *resource.RouteSpec) {
	spec.DisplayName = strings.TrimSpace(spec.DisplayName)
	for i := range spec.GatewayRefs {
		if gatewayID, valid := resourceconfig.NormalizeID(spec.GatewayRefs[i]); valid {
			spec.GatewayRefs[i] = gatewayID
		}
	}
	slices.Sort(spec.GatewayRefs)
	for i := range spec.Hostnames {
		hostname, ok := hostnameutil.Normalize(strings.TrimSpace(spec.Hostnames[i]))
		if ok {
			spec.Hostnames[i] = hostname
		}
	}
	slices.Sort(spec.Hostnames)
	spec.Match.Path.Value = strings.TrimSpace(spec.Match.Path.Value)
	for i := range spec.Match.Methods {
		spec.Match.Methods[i] = strings.ToUpper(strings.TrimSpace(spec.Match.Methods[i]))
	}
	slices.Sort(spec.Match.Methods)
	for i := range spec.Match.Headers {
		spec.Match.Headers[i].Name = httpheader.NormalizeName(spec.Match.Headers[i].Name)
		spec.Match.Headers[i].Value = httpheader.NormalizeValue(spec.Match.Headers[i].Value)
	}
	slices.SortFunc(spec.Match.Headers, func(left, right resource.HeaderMatch) int {
		return cmp.Or(
			strings.Compare(left.Name, right.Name),
			strings.Compare(left.Value, right.Value),
		)
	})
	for i := range spec.UpstreamRefs {
		if upstreamID, valid := resourceconfig.NormalizeID(spec.UpstreamRefs[i].Name); valid {
			spec.UpstreamRefs[i].Name = upstreamID
		}
	}
	slices.SortFunc(spec.UpstreamRefs, func(left, right resource.UpstreamRef) int {
		return cmp.Or(
			strings.Compare(left.Name, right.Name),
			cmp.Compare(left.Weight, right.Weight),
		)
	})
	if spec.AI != nil {
		for i := range spec.AI.Models {
			for j := range spec.AI.Models[i].Targets {
				target := &spec.AI.Models[i].Targets[j]
				if upstreamID, valid := resourceconfig.NormalizeID(target.UpstreamRef); valid {
					target.UpstreamRef = upstreamID
				}
			}
			slices.SortFunc(spec.AI.Models[i].Targets, func(left, right resource.AIModelTarget) int {
				return cmp.Or(
					strings.Compare(left.UpstreamRef, right.UpstreamRef),
					strings.Compare(left.Model, right.Model),
					cmp.Compare(left.Weight, right.Weight),
				)
			})
		}
		slices.SortFunc(spec.AI.Models, func(left, right resource.AIModel) int {
			return strings.Compare(left.Name, right.Name)
		})
	}
	spec.HostRewrite.Hostname = strings.ToLower(strings.TrimSpace(spec.HostRewrite.Hostname))
	canonicalizeHeaderModifier(spec.RequestHeaderModifier)
	canonicalizeHeaderModifier(spec.ResponseHeaderModifier)
}

func canonicalizeHeaderModifier(modifier *resource.HeaderModifier) {
	if modifier == nil {
		return
	}
	for i := range modifier.Set {
		modifier.Set[i].Name = httpheader.NormalizeName(modifier.Set[i].Name)
		modifier.Set[i].Value = httpheader.NormalizeValue(modifier.Set[i].Value)
	}
	slices.SortFunc(modifier.Set, func(left, right resource.HeaderValue) int {
		return cmp.Or(
			strings.Compare(left.Name, right.Name),
			strings.Compare(left.Value, right.Value),
		)
	})
	for i := range modifier.Add {
		modifier.Add[i].Name = httpheader.NormalizeName(modifier.Add[i].Name)
		modifier.Add[i].Value = httpheader.NormalizeValue(modifier.Add[i].Value)
	}
	slices.SortFunc(modifier.Add, func(left, right resource.HeaderValue) int {
		return cmp.Or(
			strings.Compare(left.Name, right.Name),
			strings.Compare(left.Value, right.Value),
		)
	})
	for i := range modifier.Remove {
		modifier.Remove[i] = httpheader.NormalizeName(modifier.Remove[i])
	}
	slices.Sort(modifier.Remove)
}
