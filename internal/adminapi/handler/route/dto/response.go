package dto

import (
	"encoding/json"
	"time"

	routeservice "github.com/lgc202/ingate/internal/adminapi/service/route"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FromListResult 转换 Route 列表用例结果为控制台响应
func FromListResult(result *routeservice.ListResult) *ListResponse {
	routes := make([]Route, 0, len(result.Routes))
	for i := range result.Routes {
		routes = append(routes, routeFromResource(&result.Routes[i]))
	}
	return &ListResponse{Routes: routes}
}

// PolicyCapabilities 返回当前后端支持的路由策略能力
func PolicyCapabilities() *PolicyCapabilitiesResponse {
	return &PolicyCapabilitiesResponse{Policies: builtinPolicyOptions()}
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

func builtinPolicyOptions() []PolicyOption {
	return []PolicyOption{
		{
			Name:    routePolicyRequestHeaderRewriteName,
			Meta:    "在转发到上游前设置、追加或删除请求 Header，常用于租户标识、灰度标记和上游兼容",
			Enabled: false,
			Params: []PolicyParam{
				{Key: paramSetHeadersOn, Label: "写入 Header 名称", InputType: "text", DefaultValue: "", Placeholder: "多个名称用逗号分隔", Required: true},
				{Key: paramHeaderValue, Label: "Header 值", InputType: "text", DefaultValue: "", Placeholder: "请输入要写入的 Header 值", Required: true},
				{Key: paramRemoveHeadersOn, Label: "删除 Header 名称", InputType: "text", DefaultValue: "", Placeholder: "多个名称用逗号分隔"},
			},
		},
		{
			Name:    routePolicyTimeoutName,
			Meta:    "设置当前路由从进入网关到返回响应的最长时间，包含失败重试过程",
			Enabled: false,
			Params: []PolicyParam{
				{Key: paramTimeoutMillis, Label: "请求总超时", InputType: "number", DefaultValue: "30000", Required: true, Unit: "ms", Min: minRouteTimeoutMillis, Max: maxRouteTimeoutMillis},
			},
		},
		{
			Name:    routePolicyRetryName,
			Meta:    "针对 5xx、连接失败等上游异常进行有限重试；单次尝试超时不能超过请求总超时",
			Enabled: false,
			Params: []PolicyParam{
				{Key: paramRetryAttempts, Label: "重试次数", InputType: "number", DefaultValue: "2", Required: true, Unit: "次", Min: minRetryAttempts, Max: maxRetryAttempts},
				{Key: paramPerTryTimeoutMillis, Label: "单次尝试超时", InputType: "number", DefaultValue: "1000", Required: true, Unit: "ms", Min: minPerTryTimeoutMillis, Max: maxPerTryTimeoutMillis},
			},
		},
	}
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
