package route

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

type routeInput struct {
	name                   string
	enabled                bool
	gatewayIDs             []string
	hostnames              []string
	match                  *adminv1.RouteMatch
	upstreams              []*adminv1.RouteUpstream
	hostRewrite            *adminv1.HostRewrite
	requestHeaderModifier  *adminv1.HeaderModifier
	responseHeaderModifier *adminv1.HeaderModifier
	timeout                *adminv1.RouteTimeout
	retry                  *adminv1.RouteRetry
}

// buildRouteSpec 校验请求自身语义并构造声明式 Route 配置
func buildRouteSpec(input routeInput) (resource.RouteSpec, error) {
	name := strings.TrimSpace(input.name)
	if name == "" {
		return resource.RouteSpec{}, adminservice.BadRequest("路由名称不能为空")
	}
	gatewayIDs, err := buildGatewayRefs(input.gatewayIDs)
	if err != nil {
		return resource.RouteSpec{}, err
	}
	hostnames, err := buildHostnames(input.hostnames)
	if err != nil {
		return resource.RouteSpec{}, err
	}
	match, err := buildRouteMatch(input.match)
	if err != nil {
		return resource.RouteSpec{}, err
	}

	spec := resource.RouteSpec{
		DisplayName: name,
		Enabled:     input.enabled,
		GatewayRefs: gatewayIDs,
		Hostnames:   hostnames,
		Match:       match,
	}
	if err := buildForwarding(&spec, input.upstreams); err != nil {
		return resource.RouteSpec{}, err
	}
	spec.HostRewrite, err = buildHostRewrite(input.hostRewrite)
	if err != nil {
		return resource.RouteSpec{}, err
	}
	if input.requestHeaderModifier != nil {
		spec.RequestHeaderModifier, err = buildHeaderModifier(input.requestHeaderModifier)
		if err != nil {
			return resource.RouteSpec{}, err
		}
	}
	if input.responseHeaderModifier != nil {
		spec.ResponseHeaderModifier, err = buildHeaderModifier(input.responseHeaderModifier)
		if err != nil {
			return resource.RouteSpec{}, err
		}
	}
	if input.timeout != nil {
		spec.Timeout = &resource.RouteTimeout{RequestMillis: int(input.timeout.GetRequestMillis())}
	}
	if input.retry != nil {
		spec.Retry = &resource.RouteRetry{
			Attempts:            int(input.retry.GetAttempts()),
			PerTryTimeoutMillis: int(input.retry.GetPerTryTimeoutMillis()),
		}
	}
	if err := validateRouteBehavior(spec); err != nil {
		return resource.RouteSpec{}, err
	}
	return spec, nil
}

func buildHostRewrite(input *adminv1.HostRewrite) (*resource.HostRewrite, error) {
	if input == nil {
		return &resource.HostRewrite{Mode: resource.HostRewriteServiceAddress}, nil
	}

	hostname := strings.TrimSpace(input.GetHostname())
	switch input.GetMode() {
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_ADDRESS:
		if hostname != "" {
			return nil, adminservice.BadRequest("使用服务地址时不能填写自定义主机名")
		}
		return &resource.HostRewrite{Mode: resource.HostRewriteServiceAddress}, nil
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE:
		if hostname != "" {
			return nil, adminservice.BadRequest("保留请求主机时不能填写自定义主机名")
		}
		return &resource.HostRewrite{Mode: resource.HostRewritePreserve}, nil
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM:
		normalized, ok := hostnameutil.Normalize(hostname)
		if !ok || normalized == "*" {
			return nil, adminservice.BadRequest("自定义主机名格式不正确")
		}
		return &resource.HostRewrite{Mode: resource.HostRewriteCustom, Hostname: normalized}, nil
	default:
		return nil, adminservice.BadRequest("请选择转发请求使用的主机名")
	}
}

func buildGatewayRefs(inputs []string) ([]string, error) {
	if len(inputs) == 0 {
		return nil, adminservice.BadRequest("至少需要选择一个网关")
	}
	refs := make([]string, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		id := strings.TrimSpace(input)
		if id == "" {
			return nil, adminservice.BadRequest("网关 ID 不能为空")
		}
		if _, exists := seen[id]; exists {
			return nil, adminservice.BadRequest("同一个网关只能选择一次")
		}
		seen[id] = struct{}{}
		refs = append(refs, id)
	}
	return refs, nil
}

func buildHostnames(inputs []string) ([]string, error) {
	hostnames := make([]string, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		hostname, ok := hostnameutil.Normalize(strings.TrimSpace(input))
		if !ok || hostname == "*" {
			return nil, adminservice.BadRequest("路由域名格式不正确，留空列表表示不限制域名")
		}
		if _, exists := seen[hostname]; exists {
			return nil, adminservice.BadRequest("路由域名不能重复")
		}
		seen[hostname] = struct{}{}
		hostnames = append(hostnames, hostname)
	}
	return hostnames, nil
}
