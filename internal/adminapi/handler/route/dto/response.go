package dto

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FromWorkspaceResult 转换 Route 工作区用例结果为控制台响应
func FromWorkspaceResult(result *routeservice.WorkspaceResult) *WorkspaceResponse {
	routes := make([]Route, 0, len(result.Routes))
	for i := range result.Routes {
		routes = append(routes, routeFromResource(&result.Routes[i]))
	}

	composer := composerFromResources(result.Gateways, result.Upstreams, result.Routes)
	return &WorkspaceResponse{
		Routes:         routes,
		Composer:       composer,
		PublishPreview: defaultPublishPreview(composer),
		Detail:         defaultDetail(),
	}
}

// FromRouteResult 转换单个 Route 用例结果为控制台响应
func FromRouteResult(result *routeservice.RouteResult) *Route {
	item := routeFromResource(result.Route)
	return &item
}

func routeFromResource(route *resource.Route) Route {
	rule := firstRule(route)
	policyBindings := policyBindings(route.Annotations)

	return Route{
		ID:             route.Name,
		Version:        route.ResourceVersion,
		Methods:        methods(rule.Methods),
		Path:           rule.PathPrefix,
		GatewayNames:   route.Spec.ParentRefs,
		Hostnames:      route.Spec.Hostnames,
		ServiceName:    firstUpstreamName(rule),
		PolicyCount:    len(policyBindings),
		PolicyBindings: policyBindings,
		Traffic:        "-",
		SuccessRate:    "-",
		Enabled:        enabled(route.Annotations),
		RuntimeStatus:  runtimeStatus(),
		LastChangedAt:  lastChangedAt(route.ObjectMeta),
	}
}

func composerFromResources(gateways []resource.Gateway, upstreams []resource.Upstream, routes []resource.Route) Composer {
	gatewayNames := make([]string, 0, len(gateways))
	for _, gateway := range gateways {
		gatewayNames = append(gatewayNames, gateway.Name)
	}
	sort.Strings(gatewayNames)

	targets := targetOptions(upstreams, routes)
	serviceName := ""
	if len(targets) > 0 {
		serviceName = targets[0].Name
	}
	selectedGateways := []string{}
	if len(gatewayNames) > 0 {
		selectedGateways = []string{gatewayNames[0]}
	}

	composer := Composer{
		Methods:      []HTTPMethod{},
		Path:         "/",
		GatewayNames: selectedGateways,
		Hostnames:    []string{},
		ServiceName:  serviceName,
		PolicyCount:  0,
		RateLimit:    "",
		Validations:  []string{},
		Targets:      targets,
		Policies:     []PolicyOption{},
	}

	if len(routes) == 0 {
		return composer
	}

	route := routeFromResource(&routes[0])
	composer.Methods = route.Methods
	composer.Path = route.Path
	composer.GatewayNames = route.GatewayNames
	composer.Hostnames = route.Hostnames
	composer.ServiceName = route.ServiceName
	composer.PolicyCount = route.PolicyCount
	return composer
}

func targetOptions(upstreams []resource.Upstream, routes []resource.Route) []TargetOption {
	targets := make([]TargetOption, 0, len(upstreams))
	for i := range upstreams {
		upstream := upstreams[i]
		targets = append(targets, TargetOption{
			Name:             upstream.Name,
			Type:             serviceType(upstream.Annotations),
			Endpoint:         upstreamEndpoint(upstream),
			Meta:             upstreamMeta(upstream),
			HealthStatus:     healthStatus(upstream.Status),
			ReferencedRoutes: referencedRoutes(upstream.Name, routes),
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets
}

func firstRule(route *resource.Route) resource.RouteRule {
	if len(route.Spec.Rules) == 0 {
		return resource.RouteRule{}
	}
	return route.Spec.Rules[0]
}

func methods(items []string) []HTTPMethod {
	methods := make([]HTTPMethod, 0, len(items))
	for _, item := range items {
		methods = append(methods, HTTPMethod(item))
	}
	return methods
}

func firstUpstreamName(rule resource.RouteRule) string {
	if len(rule.UpstreamRefs) == 0 {
		return ""
	}
	return rule.UpstreamRefs[0].Name
}

func policyBindings(annotations map[string]string) []PolicyBindingRequest {
	value := annotation(annotations, resource.AnnotationRoutePolicyBindings)
	if value == "" {
		return []PolicyBindingRequest{}
	}
	items := []PolicyBindingRequest{}
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return []PolicyBindingRequest{}
	}
	return items
}

func enabled(annotations map[string]string) bool {
	value := annotation(annotations, resource.AnnotationRouteEnabled)
	if value == "" {
		return true
	}
	return value != "false"
}

func runtimeStatus() string {
	// Route 是否生效需要运行时回执确认，当前管理面只展示 unknown
	return "unknown"
}

func serviceType(annotations map[string]string) string {
	value := annotation(annotations, resource.AnnotationUpstreamServiceType)
	if value == "" {
		return "application"
	}
	return value
}

func upstreamEndpoint(upstream resource.Upstream) string {
	if len(upstream.Spec.Endpoints) == 0 {
		return ""
	}
	endpoint := upstream.Spec.Endpoints[0]
	summary := fmt.Sprintf("%s:%d", endpoint.Address, endpoint.Port)
	if len(upstream.Spec.Endpoints) > 1 {
		summary = fmt.Sprintf("%s 等 %d 个端点", summary, len(upstream.Spec.Endpoints))
	}
	return summary
}

func upstreamMeta(upstream resource.Upstream) string {
	count := len(upstream.Spec.Endpoints)
	if count == 0 {
		return "未配置端点"
	}
	return fmt.Sprintf("%d 个端点", count)
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

func referencedRoutes(upstreamName string, routes []resource.Route) int {
	count := 0
	for _, route := range routes {
		matched := false
		for _, rule := range route.Spec.Rules {
			for _, ref := range rule.UpstreamRefs {
				if ref.Name == upstreamName {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			count++
		}
	}
	return count
}

func defaultPublishPreview(composer Composer) PublishPreview {
	return PublishPreview{
		Title:    fmt.Sprintf("%s %s", formatMethods(composer.Methods), composer.Path),
		Subtitle: fmt.Sprintf("目标服务 %s", composer.ServiceName),
		Diffs:    []Diff{},
	}
}

func defaultDetail() Detail {
	return Detail{
		Title: "路由详情",
		Tabs: map[string][]KeyValue{
			"overview": []KeyValue{},
			"match":    []KeyValue{},
			"target":   []KeyValue{},
			"events":   []KeyValue{},
		},
	}
}

func formatMethods(methods []HTTPMethod) string {
	if len(methods) == 0 {
		return "全部方法"
	}
	values := make([]string, 0, len(methods))
	for _, method := range methods {
		values = append(values, string(method))
	}
	return strings.Join(values, "、")
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
