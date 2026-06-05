package dto

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	upstreamservice "github.com/lgc202/ingate/internal/adminapi/service/upstream"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FromListResult 转换 Upstream 列表用例结果为控制台服务列表响应
func FromListResult(result *upstreamservice.ListResult) *ListResponse {
	services := make([]Upstream, 0, len(result.Upstreams))
	for i := range result.Upstreams {
		services = append(services, upstreamFromResource(&result.Upstreams[i], result.Routes))
	}

	return &ListResponse{
		Services:  services,
		Health:    []CountSegment{},
		Incidents: []ServiceIncident{},
	}
}

// FromUpstreamResult 转换单个 Upstream 用例结果为控制台服务响应
func FromUpstreamResult(result *upstreamservice.UpstreamResult) *Upstream {
	item := upstreamFromResource(result.Upstream, result.Routes)
	return &item
}

func upstreamFromResource(upstream *resource.Upstream, routes []resource.Route) Upstream {
	endpoints := endpointRequests(upstream)

	return Upstream{
		ID:               upstream.Name,
		Version:          upstream.ResourceVersion,
		Name:             upstreamDisplayName(upstream),
		Type:             serviceType(upstream.Annotations),
		Endpoint:         endpointSummary(endpoints),
		Instances:        instanceSummary(endpoints),
		HealthStatus:     healthStatus(upstream.Status),
		RuntimeStatus:    runtimeStatus(),
		ReferencedRoutes: referencedRoutes(upstream.Name, routes),
		Traffic:          "-",
		SuccessRate:      "-",
		CreatedAt:        createdAt(upstream.ObjectMeta),
		Endpoints:        endpoints,
	}
}

func upstreamDisplayName(upstream *resource.Upstream) string {
	if upstream.Spec.DisplayName != "" {
		return upstream.Spec.DisplayName
	}
	return upstream.Name
}

func serviceType(annotations map[string]string) ServiceType {
	switch ServiceType(annotation(annotations, resource.AnnotationUpstreamServiceType)) {
	case ServiceTypeModel:
		return ServiceTypeModel
	case ServiceTypeAgent:
		return ServiceTypeAgent
	case ServiceTypeMCP:
		return ServiceTypeMCP
	default:
		return ServiceTypeApplication
	}
}

func endpointRequests(upstream *resource.Upstream) []EndpointRequest {
	if endpoints := annotationEndpoints(upstream.Annotations); len(endpoints) > 0 {
		return endpoints
	}

	endpoints := make([]EndpointRequest, 0, len(upstream.Spec.Endpoints))
	for i, endpoint := range upstream.Spec.Endpoints {
		endpoints = append(endpoints, EndpointRequest{
			ID:      fmt.Sprintf("%s-endpoint-%d", upstream.Name, i+1),
			Address: endpoint.Address,
			Port:    strconv.Itoa(endpoint.Port),
			Weight:  "100",
			Enabled: true,
		})
	}
	return endpoints
}

func annotationEndpoints(annotations map[string]string) []EndpointRequest {
	value := annotation(annotations, resource.AnnotationUpstreamEndpoints)
	if value == "" {
		return nil
	}
	endpoints := []EndpointRequest{}
	if err := json.Unmarshal([]byte(value), &endpoints); err != nil {
		return nil
	}
	return endpoints
}

func endpointSummary(endpoints []EndpointRequest) string {
	visible := make([]EndpointRequest, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint.Enabled {
			visible = append(visible, endpoint)
		}
	}
	if len(visible) == 0 {
		visible = endpoints
	}
	if len(visible) == 0 {
		return "-"
	}

	summary := fmt.Sprintf("%s:%s", visible[0].Address, visible[0].Port)
	if len(visible) > 1 {
		summary = fmt.Sprintf("%s 等 %d 个端点", summary, len(visible))
	}
	return summary
}

func instanceSummary(endpoints []EndpointRequest) string {
	enabled := 0
	for _, endpoint := range endpoints {
		if endpoint.Enabled {
			enabled++
		}
	}
	return fmt.Sprintf("%d/%d", enabled, len(endpoints))
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

func runtimeStatus() string {
	// RuntimeSnapshot 不能证明运行时已经应用，服务页先统一展示 unknown
	return "unknown"
}

func referencedRoutes(upstreamID string, routes []resource.Route) int {
	count := 0
	for _, route := range routes {
		if routeReferencesUpstream(route, upstreamID) {
			count++
		}
	}
	return count
}

func routeReferencesUpstream(route resource.Route, upstreamID string) bool {
	for _, rule := range route.Spec.Rules {
		if slices.ContainsFunc(rule.UpstreamRefs, func(ref resource.UpstreamRef) bool {
			return ref.Name == upstreamID
		}) {
			return true
		}
	}
	return false
}

func annotation(annotations map[string]string, key string) string {
	if annotations == nil {
		return ""
	}
	return annotations[key]
}

func createdAt(metadata metav1.ObjectMeta) string {
	if metadata.CreationTimestamp.IsZero() {
		return ""
	}
	return metadata.CreationTimestamp.UTC().Format(time.RFC3339)
}
