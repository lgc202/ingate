package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

const (
	DefaultAllowedRouteKind         = "Route"
	DefaultBackendRefWeight   int32 = 100
	DefaultLoadBalancerPolicy       = "RoundRobin"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_Gateway(obj *Gateway) {
	if obj == nil {
		return
	}

	if obj.Spec.AllowedRoutes == nil {
		obj.Spec.AllowedRoutes = &AllowedRoutesSpec{Kinds: []string{DefaultAllowedRouteKind}}
		return
	}

	if len(obj.Spec.AllowedRoutes.Kinds) == 0 {
		obj.Spec.AllowedRoutes.Kinds = []string{DefaultAllowedRouteKind}
	}
}

func SetDefaults_Route(obj *Route) {
	if obj == nil {
		return
	}

	for i := range obj.Spec.Rules {
		for j := range obj.Spec.Rules[i].BackendRefs {
			if obj.Spec.Rules[i].BackendRefs[j].Weight == 0 {
				obj.Spec.Rules[i].BackendRefs[j].Weight = DefaultBackendRefWeight
			}
		}
	}
}

func SetDefaults_Backend(obj *Backend) {
	if obj == nil {
		return
	}

	if obj.Spec.Protocol == "" {
		obj.Spec.Protocol = BackendProtocolHTTP
	}

	if obj.Spec.LoadBalance == nil {
		obj.Spec.LoadBalance = &LoadBalanceSpec{Policy: DefaultLoadBalancerPolicy}
		return
	}

	if obj.Spec.LoadBalance.Policy == "" {
		obj.Spec.LoadBalance.Policy = DefaultLoadBalancerPolicy
	}
}

func SetDefaults_ResolvedGateway(obj *ResolvedGateway) {
	if obj == nil {
		return
	}
}
