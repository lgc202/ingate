package biz

import (
	"context"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

type OverviewService struct {
	store *store.APIServerStore
}

func NewOverviewService(store *store.APIServerStore) *OverviewService {
	return &OverviewService{store: store}
}

func (s *OverviewService) GetOverview(ctx context.Context) (dto.OverviewResponse, error) {
	gateways, err := s.store.ListGateways(ctx)
	if err != nil {
		return dto.OverviewResponse{}, err
	}
	routes, err := s.store.ListRoutes(ctx)
	if err != nil {
		return dto.OverviewResponse{}, err
	}
	backends, err := s.store.ListBackends(ctx)
	if err != nil {
		return dto.OverviewResponse{}, err
	}
	authPolicies, err := s.store.ListAuthPolicies(ctx)
	if err != nil {
		return dto.OverviewResponse{}, err
	}
	trafficPolicies, err := s.store.ListTrafficPolicies(ctx)
	if err != nil {
		return dto.OverviewResponse{}, err
	}

	return buildOverviewResponse(gateways.Items, routes.Items, backends.Items, authPolicies.Items, trafficPolicies.Items), nil
}

func buildOverviewResponse(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.Route,
	backends []gatewayv1alpha1.Backend,
	authPolicies []policyv1alpha1.AuthPolicy,
	trafficPolicies []policyv1alpha1.TrafficPolicy,
) dto.OverviewResponse {
	gatewayNames := gatewayNameSet(gateways)
	backendNames := backendNameSet(backends)
	routeNames := routeNameSet(routes)
	chains := make([]dto.OverviewChain, 0, len(routes))
	unresolved := int32(0)
	hasRisk := false

	for _, route := range routes {
		gatewayName := firstRouteGatewayName(route)
		backendName := firstRouteBackendName(route)
		chainStatus := routeStatus(route)
		if gatewayName != "" && gatewayName != "-" {
			if _, exists := gatewayNames[gatewayName]; !exists {
				unresolved++
				chainStatus = resourceStatusError
			}
		}
		if backendName != "" && backendName != "-" {
			if _, exists := backendNames[backendName]; !exists {
				unresolved++
				chainStatus = resourceStatusError
			}
		}
		if chainStatus == resourceStatusWarning || chainStatus == resourceStatusError {
			hasRisk = true
		}
		chains = append(chains, dto.OverviewChain{
			GatewayName: gatewayName,
			RouteName:   route.Name,
			RouteHost:   firstRouteHostname(route),
			RoutePath:   firstRoutePath(route),
			BackendName: backendName,
			Status:      chainStatus,
		})
	}

	unresolved += int32(countUnresolvedPolicyRefs(authPolicies, trafficPolicies, gatewayNames, routeNames, backendNames))
	status := resourceStatusHealthy
	if unresolved > 0 || hasRisk {
		status = resourceStatusWarning
	}

	return dto.OverviewResponse{
		Summary: dto.OverviewSummary{
			GatewayCount:       int32(len(gateways)),
			RouteCount:         int32(len(routes)),
			BackendCount:       int32(len(backends)),
			PolicyCount:        int32(len(authPolicies) + len(trafficPolicies)),
			UnresolvedRefCount: unresolved,
			ControlPlaneStatus: status,
		},
		Chains: chains,
	}
}

func gatewayNameSet(gateways []gatewayv1alpha1.Gateway) map[string]struct{} {
	items := make(map[string]struct{}, len(gateways))
	for _, gateway := range gateways {
		items[gateway.Name] = struct{}{}
	}
	return items
}

func backendNameSet(backends []gatewayv1alpha1.Backend) map[string]struct{} {
	items := make(map[string]struct{}, len(backends))
	for _, backend := range backends {
		items[backend.Name] = struct{}{}
	}
	return items
}

func routeNameSet(routes []gatewayv1alpha1.Route) map[string]struct{} {
	items := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		items[route.Name] = struct{}{}
	}
	return items
}

func firstRouteGatewayName(route gatewayv1alpha1.Route) string {
	if len(route.Spec.ParentRefs) == 0 {
		return "-"
	}
	return route.Spec.ParentRefs[0].Name
}

func firstRouteBackendName(route gatewayv1alpha1.Route) string {
	for _, rule := range route.Spec.Rules {
		if len(rule.BackendRefs) > 0 {
			return rule.BackendRefs[0].Name
		}
	}
	return "-"
}

func firstRouteHostname(route gatewayv1alpha1.Route) string {
	if len(route.Spec.Hostnames) == 0 {
		return "*"
	}
	return route.Spec.Hostnames[0]
}

func firstRoutePath(route gatewayv1alpha1.Route) string {
	for _, rule := range route.Spec.Rules {
		if len(rule.Matches) > 0 && rule.Matches[0].Path != nil {
			return rule.Matches[0].Path.Value
		}
	}
	return "/"
}

func countUnresolvedPolicyRefs(
	authPolicies []policyv1alpha1.AuthPolicy,
	trafficPolicies []policyv1alpha1.TrafficPolicy,
	gatewayNames map[string]struct{},
	routeNames map[string]struct{},
	backendNames map[string]struct{},
) int {
	count := 0
	for _, policy := range authPolicies {
		count += countMissingTargets(policy.Spec.TargetRefs, gatewayNames, routeNames, backendNames)
	}
	for _, policy := range trafficPolicies {
		count += countMissingTargets(policy.Spec.TargetRefs, gatewayNames, routeNames, backendNames)
	}
	return count
}

func countMissingTargets(
	refs []policyv1alpha1.TargetReference,
	gatewayNames map[string]struct{},
	routeNames map[string]struct{},
	backendNames map[string]struct{},
) int {
	count := 0
	for _, ref := range refs {
		if ref.Name == "" {
			continue
		}
		switch ref.Kind {
		case resourceKindGateway:
			if _, exists := gatewayNames[ref.Name]; !exists {
				count++
			}
		case resourceKindRoute:
			if _, exists := routeNames[ref.Name]; !exists {
				count++
			}
		case resourceKindBackend:
			if _, exists := backendNames[ref.Name]; !exists {
				count++
			}
		}
	}
	return count
}
