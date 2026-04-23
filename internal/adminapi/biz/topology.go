package biz

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lgc202/ingate/internal/adminapi/handler/dto"
	"github.com/lgc202/ingate/internal/adminapi/store"
	gatewayv1alpha1 "github.com/lgc202/ingate/pkg/apis/gateway/v1alpha1"
	policyv1alpha1 "github.com/lgc202/ingate/pkg/apis/policy/v1alpha1"
)

const (
	resourceKindGateway       = "Gateway"
	resourceKindRoute         = "Route"
	resourceKindBackend       = "Backend"
	resourceKindAuthPolicy    = "AuthPolicy"
	resourceKindTrafficPolicy = "TrafficPolicy"

	resourceStatusHealthy = "Healthy"
	resourceStatusPending = "Pending"
	resourceStatusWarning = "Warning"
	resourceStatusError   = "Error"
)

type TopologyService struct {
	store store.Store
}

func NewTopologyService(store store.Store) *TopologyService {
	return &TopologyService{store: store}
}

func (s *TopologyService) GetTopology(ctx context.Context) (dto.TopologyResponse, error) {
	gateways, err := s.store.ListGateways(ctx)
	if err != nil {
		return dto.TopologyResponse{}, err
	}
	routes, err := s.store.ListRoutes(ctx)
	if err != nil {
		return dto.TopologyResponse{}, err
	}
	backends, err := s.store.ListBackends(ctx)
	if err != nil {
		return dto.TopologyResponse{}, err
	}
	authPolicies, err := s.store.ListAuthPolicies(ctx)
	if err != nil {
		return dto.TopologyResponse{}, err
	}
	trafficPolicies, err := s.store.ListTrafficPolicies(ctx)
	if err != nil {
		return dto.TopologyResponse{}, err
	}

	return buildTopologyResponse(gateways.Items, routes.Items, backends.Items, authPolicies.Items, trafficPolicies.Items), nil
}

func buildTopologyResponse(
	gateways []gatewayv1alpha1.Gateway,
	routes []gatewayv1alpha1.Route,
	backends []gatewayv1alpha1.Backend,
	authPolicies []policyv1alpha1.AuthPolicy,
	trafficPolicies []policyv1alpha1.TrafficPolicy,
) dto.TopologyResponse {
	nodes := make([]dto.TopologyNode, 0, len(gateways)+len(routes)+len(backends)+len(authPolicies)+len(trafficPolicies))
	edges := make([]dto.TopologyEdge, 0)

	for i := range gateways {
		gateway := gateways[i]
		nodes = append(nodes, newTopologyNode(topologyNodeID(resourceKindGateway, gateway.Name), gateway.Name, resourceKindGateway, statusFromConditions(gateway.Status.Conditions)))
	}
	for i := range routes {
		route := routes[i]
		nodes = append(nodes, newTopologyNode(topologyNodeID(resourceKindRoute, route.Name), route.Name, resourceKindRoute, routeStatus(route)))
		for _, parent := range route.Spec.ParentRefs {
			if parent.Name != "" {
				edges = append(edges, newTopologyEdge(topologyNodeID(resourceKindGateway, parent.Name), topologyNodeID(resourceKindRoute, route.Name), "serves"))
			}
		}
		for _, rule := range route.Spec.Rules {
			for _, backendRef := range rule.BackendRefs {
				if backendRef.Name != "" {
					edges = append(edges, newTopologyEdge(topologyNodeID(resourceKindRoute, route.Name), topologyNodeID(resourceKindBackend, backendRef.Name), "forwards"))
				}
			}
		}
	}
	for i := range backends {
		backend := backends[i]
		nodes = append(nodes, newTopologyNode(topologyNodeID(resourceKindBackend, backend.Name), backend.Name, resourceKindBackend, statusFromConditions(backend.Status.Conditions)))
	}
	for i := range authPolicies {
		policy := authPolicies[i]
		nodes = append(nodes, newTopologyNode(topologyNodeID(resourceKindAuthPolicy, policy.Name), policy.Name, resourceKindAuthPolicy, statusFromConditions(policy.Status.Conditions)))
		edges = append(edges, policyTargetEdges(resourceKindAuthPolicy, policy.Name, policy.Spec.TargetRefs, "auth")...)
	}
	for i := range trafficPolicies {
		policy := trafficPolicies[i]
		nodes = append(nodes, newTopologyNode(topologyNodeID(resourceKindTrafficPolicy, policy.Name), policy.Name, resourceKindTrafficPolicy, statusFromConditions(policy.Status.Conditions)))
		edges = append(edges, policyTargetEdges(resourceKindTrafficPolicy, policy.Name, policy.Spec.TargetRefs, "traffic")...)
	}

	edges = dedupeTopologyEdges(edges)
	nodes = ensureEdgeEndpointNodes(dedupeTopologyNodes(nodes), edges)
	return dto.TopologyResponse{Nodes: filterConnectedTopologyNodes(nodes, edges), Edges: edges}
}

func newTopologyNode(id, label, kind, status string) dto.TopologyNode {
	return dto.TopologyNode{ID: id, Label: label, Kind: kind, Status: status}
}

func newTopologyEdge(from, to, label string) dto.TopologyEdge {
	return dto.TopologyEdge{From: from, To: to, Label: label}
}

func topologyNodeID(kind, name string) string {
	switch kind {
	case resourceKindGateway:
		return "gateway-" + name
	case resourceKindRoute:
		return "route-" + name
	case resourceKindBackend:
		return "backend-" + name
	case resourceKindAuthPolicy:
		return "auth-" + name
	case resourceKindTrafficPolicy:
		return "traffic-" + name
	default:
		return fmt.Sprintf("%s-%s", kind, name)
	}
}

func routeStatus(route gatewayv1alpha1.Route) string {
	conditions := make([]metav1.Condition, 0, len(route.Status.Conditions))
	conditions = append(conditions, route.Status.Conditions...)
	for _, parent := range route.Status.Parents {
		conditions = append(conditions, parent.Conditions...)
	}
	return statusFromConditions(conditions)
}

func statusFromConditions(conditions []metav1.Condition) string {
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionFalse {
			return resourceStatusError
		}
	}
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionUnknown {
			return resourceStatusWarning
		}
	}
	for _, condition := range conditions {
		if condition.Status == metav1.ConditionTrue {
			return resourceStatusHealthy
		}
	}
	return resourceStatusPending
}

func policyTargetEdges(policyKind, policyName string, refs []policyv1alpha1.TargetReference, label string) []dto.TopologyEdge {
	edges := make([]dto.TopologyEdge, 0, len(refs))
	policyID := topologyNodeID(policyKind, policyName)
	for _, ref := range refs {
		switch ref.Kind {
		case resourceKindGateway:
			edges = append(edges, newTopologyEdge(policyID, topologyNodeID(resourceKindGateway, ref.Name), label))
		case resourceKindRoute:
			edges = append(edges, newTopologyEdge(policyID, topologyNodeID(resourceKindRoute, ref.Name), label))
		case resourceKindBackend:
			edges = append(edges, newTopologyEdge(policyID, topologyNodeID(resourceKindBackend, ref.Name), label))
		}
	}
	return edges
}

func dedupeTopologyNodes(nodes []dto.TopologyNode) []dto.TopologyNode {
	seen := make(map[string]struct{}, len(nodes))
	items := make([]dto.TopologyNode, 0, len(nodes))
	for _, node := range nodes {
		if _, exists := seen[node.ID]; exists {
			continue
		}
		seen[node.ID] = struct{}{}
		items = append(items, node)
	}
	return items
}

func dedupeTopologyEdges(edges []dto.TopologyEdge) []dto.TopologyEdge {
	seen := make(map[string]struct{}, len(edges))
	items := make([]dto.TopologyEdge, 0, len(edges))
	for _, edge := range edges {
		key := edge.From + "\x00" + edge.To + "\x00" + edge.Label
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, edge)
	}
	return items
}

func filterConnectedTopologyNodes(nodes []dto.TopologyNode, edges []dto.TopologyEdge) []dto.TopologyNode {
	connected := make(map[string]struct{}, len(edges)*2)
	for _, edge := range edges {
		connected[edge.From] = struct{}{}
		connected[edge.To] = struct{}{}
	}

	items := make([]dto.TopologyNode, 0, len(nodes))
	for _, node := range nodes {
		if _, exists := connected[node.ID]; exists {
			items = append(items, node)
		}
	}
	return items
}

func ensureEdgeEndpointNodes(nodes []dto.TopologyNode, edges []dto.TopologyEdge) []dto.TopologyNode {
	seen := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		seen[node.ID] = struct{}{}
	}

	items := append([]dto.TopologyNode(nil), nodes...)
	for _, edge := range edges {
		for _, id := range []string{edge.From, edge.To} {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			items = append(items, unresolvedTopologyNode(id))
		}
	}
	return items
}

func unresolvedTopologyNode(id string) dto.TopologyNode {
	kind, label := topologyNodeKindAndLabel(id)
	return newTopologyNode(id, label, kind, resourceStatusError)
}

func topologyNodeKindAndLabel(id string) (string, string) {
	prefixes := []struct {
		prefix string
		kind   string
	}{
		{prefix: "gateway-", kind: resourceKindGateway},
		{prefix: "route-", kind: resourceKindRoute},
		{prefix: "backend-", kind: resourceKindBackend},
		{prefix: "auth-", kind: resourceKindAuthPolicy},
		{prefix: "traffic-", kind: resourceKindTrafficPolicy},
	}
	for _, item := range prefixes {
		if len(id) > len(item.prefix) && id[:len(item.prefix)] == item.prefix {
			return item.kind, id[len(item.prefix):]
		}
	}
	return resourceKindBackend, id
}
