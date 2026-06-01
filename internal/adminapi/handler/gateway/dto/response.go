package dto

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gatewayservice "github.com/lgc202/ingate/internal/adminapi/service/gateway"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	defaultRuntimeGroupID   = "default"
	defaultRuntimeGroupName = "默认运行组"
)

// FromListResult 转换 Gateway 列表用例结果为 HTTP 响应。
func FromListResult(result *gatewayservice.ListResult) *ListResponse {
	items := make([]Gateway, 0, len(result.Gateways))
	for i := range result.Gateways {
		items = append(items, gatewayFromResource(&result.Gateways[i], result.Routes, result.Upstreams, result.RuntimeSnapshots))
	}

	return &ListResponse{
		Gateways:     items,
		Certificates: []Certificate{},
	}
}

// FromGatewayResult 转换单个 Gateway 用例结果为 HTTP 响应。
func FromGatewayResult(result *gatewayservice.GatewayResult) *Gateway {
	item := gatewayFromResource(result.Gateway, result.Routes, result.Upstreams, result.RuntimeSnapshots)
	return &item
}

// FromDetailResult 转换 Gateway 详情用例结果为 HTTP 响应。
func FromDetailResult(result *gatewayservice.DetailResult) *DetailResponse {
	item := gatewayFromResource(result.Gateway, result.Routes, result.Upstreams, result.RuntimeSnapshots)
	return &DetailResponse{
		Gateway:          item,
		Routes:           routeReferences(result.Routes),
		Services:         serviceReferences(result.Upstreams),
		RuntimeSnapshots: runtimeStatuses(result.RuntimeSnapshots),
	}
}

func gatewayFromResource(gateway *resource.Gateway, routes []resource.Route, upstreams []resource.Upstream, snapshots []resource.RuntimeSnapshot) Gateway {
	matchedRoutes := gatewayRoutes(gateway.Name, routes)
	matchedUpstreams := gatewayUpstreams(matchedRoutes, upstreams)
	matchedSnapshots := gatewaySnapshots(gateway.Name, snapshots)
	hostnames := gatewayHostnames(gateway)

	return Gateway{
		ID:                    gateway.Name,
		Name:                  gateway.Name,
		Description:           annotation(gateway.Annotations, resource.AnnotationGatewayDescription),
		RuntimeGroupID:        defaultRuntimeGroupID,
		RuntimeGroupName:      defaultRuntimeGroupName,
		ListenerSummary:       listenerSummary(gateway.Spec.Listeners),
		Listeners:             listeners(gateway.Spec.Listeners),
		HostPolicy:            hostPolicy(hostnames),
		Hostnames:             hostnames,
		RouteCount:            len(matchedRoutes),
		ServiceCount:          len(matchedUpstreams),
		Enabled:               enabled(gateway.Annotations),
		RuntimeStatus:         runtimeStatus(),
		HealthStatus:          healthStatus(gateway.Status),
		LatestSnapshotVersion: latestSnapshotVersion(matchedSnapshots),
		LastChangedAt:         lastChangedAt(gateway.ObjectMeta),
	}
}

func gatewayRoutes(gatewayName string, routes []resource.Route) []resource.Route {
	matched := make([]resource.Route, 0)
	for _, route := range routes {
		if slices.Contains(route.Spec.ParentRefs, gatewayName) {
			matched = append(matched, route)
		}
	}
	return matched
}

func gatewayUpstreams(routes []resource.Route, upstreams []resource.Upstream) []resource.Upstream {
	names := map[string]struct{}{}
	for _, route := range routes {
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				names[ref.Name] = struct{}{}
			}
		}
	}
	matched := make([]resource.Upstream, 0, len(names))
	for _, upstream := range upstreams {
		if _, ok := names[upstream.Name]; ok {
			matched = append(matched, upstream)
		}
	}
	return matched
}

func gatewaySnapshots(gatewayName string, snapshots []resource.RuntimeSnapshot) []resource.RuntimeSnapshot {
	matched := make([]resource.RuntimeSnapshot, 0)
	for _, snapshot := range snapshots {
		if snapshot.Spec.Gateway == gatewayName {
			matched = append(matched, snapshot)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].CreationTimestamp.After(matched[j].CreationTimestamp.Time)
	})
	return matched
}

func listeners(items []resource.Listener) []Listener {
	listeners := make([]Listener, 0, len(items))
	for _, item := range items {
		listeners = append(listeners, Listener{
			ID:       item.Name,
			Protocol: item.Protocol,
			Port:     strconv.Itoa(item.Port),
		})
	}
	return listeners
}

func listenerSummary(listeners []resource.Listener) string {
	if len(listeners) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		parts = append(parts, fmt.Sprintf("%s:%d", listener.Protocol, listener.Port))
	}
	return strings.Join(parts, " / ")
}

func gatewayHostnames(gateway *resource.Gateway) []string {
	if hostnames := annotationHostnames(gateway.Annotations); len(hostnames) > 0 {
		return hostnames
	}

	seen := map[string]struct{}{}
	hostnames := make([]string, 0)
	for _, listener := range gateway.Spec.Listeners {
		hostname := strings.TrimSpace(listener.Hostname)
		if hostname == "" {
			continue
		}
		if _, ok := seen[hostname]; ok {
			continue
		}
		seen[hostname] = struct{}{}
		hostnames = append(hostnames, hostname)
	}
	sort.Strings(hostnames)
	return hostnames
}

func hostPolicy(hostnames []string) string {
	if len(hostnames) == 0 {
		return "不限制 Host"
	}
	return strings.Join(hostnames, "、")
}

func runtimeStatus() string {
	// RuntimeSnapshot 只能说明控制面生成过配置，不能证明运行时已经应用。
	// 等后续接入运行时回执后，再从真实回执计算 synced/failed 等状态。
	return "unknown"
}

func latestSnapshotVersion(snapshots []resource.RuntimeSnapshot) string {
	if len(snapshots) == 0 {
		return ""
	}
	return snapshots[0].Spec.Version
}

func healthStatus(status resource.ResourceStatus) string {
	for _, condition := range status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionFalse {
			return "critical"
		}
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			return "healthy"
		}
	}
	return "unknown"
}

func enabled(annotations map[string]string) bool {
	value := annotation(annotations, resource.AnnotationGatewayEnabled)
	if value == "" {
		return true
	}
	return value != "false"
}

func annotationHostnames(annotations map[string]string) []string {
	value := annotation(annotations, resource.AnnotationGatewayHostnames)
	if value == "" {
		return nil
	}

	hostnames := []string{}
	if err := json.Unmarshal([]byte(value), &hostnames); err != nil {
		return nil
	}
	return normalizedHostnames(hostnames)
}

func normalizedHostnames(hostnames []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(hostnames))
	for _, hostname := range hostnames {
		hostname = strings.TrimSpace(strings.ToLower(hostname))
		if hostname == "" {
			continue
		}
		if _, ok := seen[hostname]; ok {
			continue
		}
		seen[hostname] = struct{}{}
		normalized = append(normalized, hostname)
	}
	sort.Strings(normalized)
	return normalized
}

func annotation(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return annotations[key]
}

func lastChangedAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}

func routeReferences(routes []resource.Route) []RouteReference {
	references := make([]RouteReference, 0, len(routes))
	for _, route := range routes {
		references = append(references, RouteReference{
			ID:          route.Name,
			Name:        route.Name,
			Methods:     routeMethods(route),
			Path:        routePath(route),
			Hostnames:   route.Spec.Hostnames,
			ServiceName: routeServiceName(route),
		})
	}
	return references
}

func routeMethods(route resource.Route) []string {
	if len(route.Spec.Rules) == 0 {
		return nil
	}
	return route.Spec.Rules[0].Methods
}

func routePath(route resource.Route) string {
	if len(route.Spec.Rules) == 0 {
		return ""
	}
	return route.Spec.Rules[0].PathPrefix
}

func routeServiceName(route resource.Route) string {
	if len(route.Spec.Rules) == 0 || len(route.Spec.Rules[0].UpstreamRefs) == 0 {
		return ""
	}
	return route.Spec.Rules[0].UpstreamRefs[0].Name
}

func serviceReferences(upstreams []resource.Upstream) []ServiceReference {
	references := make([]ServiceReference, 0, len(upstreams))
	for _, upstream := range upstreams {
		references = append(references, ServiceReference{
			ID:       upstream.Name,
			Name:     upstream.Name,
			Endpoint: upstreamEndpoint(upstream),
		})
	}
	return references
}

func upstreamEndpoint(upstream resource.Upstream) string {
	if len(upstream.Spec.Endpoints) == 0 {
		return ""
	}
	endpoint := upstream.Spec.Endpoints[0]
	return fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port)
}

func runtimeStatuses(snapshots []resource.RuntimeSnapshot) []RuntimeStatus {
	statuses := make([]RuntimeStatus, 0, len(snapshots))
	for _, snapshot := range snapshots {
		statuses = append(statuses, RuntimeStatus{
			ID:        snapshot.Name,
			Name:      snapshot.Name,
			Target:    snapshot.Spec.Target,
			Version:   snapshot.Spec.Version,
			Status:    "unknown",
			CreatedAt: lastChangedAt(snapshot.ObjectMeta),
		})
	}
	return statuses
}
