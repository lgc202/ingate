package defaulting

import gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"

const (
	DefaultAllowedRouteKind   = "Route"
	DefaultBackendRefWeight   = 100
	DefaultLoadBalancerPolicy = "RoundRobin"
)

func ApplyGatewayDefaults(gateway *gatewayv1alpha1.Gateway) {
	if gateway == nil {
		return
	}

	if gateway.Spec.AllowedRoutes == nil {
		gateway.Spec.AllowedRoutes = &gatewayv1alpha1.AllowedRoutesSpec{
			Kinds: []string{DefaultAllowedRouteKind},
		}
		return
	}

	if len(gateway.Spec.AllowedRoutes.Kinds) == 0 {
		gateway.Spec.AllowedRoutes.Kinds = []string{DefaultAllowedRouteKind}
	}
}

func ApplyRouteDefaults(route *gatewayv1alpha1.Route) {
	if route == nil {
		return
	}

	for i := range route.Spec.Rules {
		for j := range route.Spec.Rules[i].BackendRefs {
			if route.Spec.Rules[i].BackendRefs[j].Weight == 0 {
				route.Spec.Rules[i].BackendRefs[j].Weight = DefaultBackendRefWeight
			}
		}
	}
}

func ApplyBackendDefaults(backend *gatewayv1alpha1.Backend) {
	if backend == nil {
		return
	}

	if backend.Spec.LoadBalance == nil {
		backend.Spec.LoadBalance = &gatewayv1alpha1.LoadBalanceSpec{
			Policy: DefaultLoadBalancerPolicy,
		}
		return
	}

	if backend.Spec.LoadBalance.Policy == "" {
		backend.Spec.LoadBalance.Policy = DefaultLoadBalancerPolicy
	}
}

func ApplyResolvedGatewayDefaults(resolvedGateway *gatewayv1alpha1.ResolvedGateway) {
	gatewayv1alpha1.SetDefaults_ResolvedGateway(resolvedGateway)
}
