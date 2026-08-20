package route

import (
	"strings"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

func createSpec(request *adminv1.CreateRouteRequest) (resource.RouteSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.RouteSpec{}, adminservice.BadRequest("路由名称不能为空")
	}
	gatewayIDs, err := gatewayRefs(request.GetGatewayIds())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	hostnames, err := routeHostnames(request.GetHostnames())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	match, err := routeMatch(request.GetMatch())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	accessMode, err := routeAccessMode(request.GetAccessMode())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	upstreams, ai, err := forwarding(request.GetUpstreams(), request.GetAi())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	rewrite, err := hostRewrite(request.GetHostRewrite())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	requestHeaders, err := headerModifier(request.GetRequestHeaderModifier())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	responseHeaders, err := headerModifier(request.GetResponseHeaderModifier())
	if err != nil {
		return resource.RouteSpec{}, err
	}

	spec := resource.RouteSpec{
		DisplayName:            name,
		Enabled:                request.GetEnabled(),
		AccessMode:             accessMode,
		GatewayRefs:            gatewayIDs,
		Hostnames:              hostnames,
		Match:                  match,
		UpstreamRefs:           upstreams,
		AI:                     ai,
		HostRewrite:            rewrite,
		RequestHeaderModifier:  requestHeaders,
		ResponseHeaderModifier: responseHeaders,
		Timeout:                routeTimeout(request.GetTimeout()),
		Retry:                  routeRetry(request.GetRetry()),
	}
	if err := validateRouteSpec(spec); err != nil {
		return resource.RouteSpec{}, err
	}
	return spec, nil
}

func updateSpec(request *adminv1.UpdateRouteRequest) (resource.RouteSpec, error) {
	name := strings.TrimSpace(request.GetName())
	if name == "" {
		return resource.RouteSpec{}, adminservice.BadRequest("路由名称不能为空")
	}
	gatewayIDs, err := gatewayRefs(request.GetGatewayIds())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	hostnames, err := routeHostnames(request.GetHostnames())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	match, err := routeMatch(request.GetMatch())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	accessMode, err := routeAccessMode(request.GetAccessMode())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	upstreams, ai, err := forwarding(request.GetUpstreams(), request.GetAi())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	rewrite, err := hostRewrite(request.GetHostRewrite())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	requestHeaders, err := headerModifier(request.GetRequestHeaderModifier())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	responseHeaders, err := headerModifier(request.GetResponseHeaderModifier())
	if err != nil {
		return resource.RouteSpec{}, err
	}

	spec := resource.RouteSpec{
		DisplayName:            name,
		Enabled:                request.GetEnabled(),
		AccessMode:             accessMode,
		GatewayRefs:            gatewayIDs,
		Hostnames:              hostnames,
		Match:                  match,
		UpstreamRefs:           upstreams,
		AI:                     ai,
		HostRewrite:            rewrite,
		RequestHeaderModifier:  requestHeaders,
		ResponseHeaderModifier: responseHeaders,
		Timeout:                routeTimeout(request.GetTimeout()),
		Retry:                  routeRetry(request.GetRetry()),
	}
	if err := validateRouteSpec(spec); err != nil {
		return resource.RouteSpec{}, err
	}
	return spec, nil
}

func routeAccessMode(mode adminv1.RouteAccessMode) (resource.RouteAccessMode, error) {
	switch mode {
	case adminv1.RouteAccessMode_ROUTE_ACCESS_PUBLIC:
		return resource.RouteAccessPublic, nil
	case adminv1.RouteAccessMode_ROUTE_ACCESS_CALLER:
		return resource.RouteAccessCaller, nil
	default:
		return "", adminservice.BadRequest("请选择访问方式")
	}
}

func hostRewrite(input *adminv1.HostRewrite) (*resource.HostRewrite, error) {
	if input == nil {
		// 控制台默认使用实际服务地址，避免把入口域名原样发送给外部服务
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

func gatewayRefs(inputs []string) ([]string, error) {
	refs := make([]string, 0, len(inputs))
	seen := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		id := strings.TrimSpace(input)
		if _, exists := seen[id]; exists {
			return nil, adminservice.BadRequest("同一个网关只能选择一次")
		}
		seen[id] = struct{}{}
		refs = append(refs, id)
	}
	return refs, nil
}

func routeHostnames(inputs []string) ([]string, error) {
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

func routeTimeout(input *adminv1.RouteTimeout) *resource.RouteTimeout {
	if input == nil {
		return nil
	}
	return &resource.RouteTimeout{RequestMillis: int(input.GetRequestMillis())}
}

func routeRetry(input *adminv1.RouteRetry) *resource.RouteRetry {
	if input == nil {
		return nil
	}
	return &resource.RouteRetry{
		Attempts:            int(input.GetAttempts()),
		PerTryTimeoutMillis: int(input.GetPerTryTimeoutMillis()),
	}
}
