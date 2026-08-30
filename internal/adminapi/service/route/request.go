package route

import (
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	hostnameutil "github.com/lgc202/ingate/internal/pkg/hostname"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

func parseRouteSpec(displayName string, config *adminv1.RouteConfig) (resource.RouteSpec, error) {
	displayName = strings.TrimSpace(displayName)
	if !resourceconfig.IsValidDisplayName(displayName) {
		return resource.RouteSpec{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由名称不能为空、不能包含控制字符且不能超过 256 字节",
		)
	}
	if config == nil {
		return resource.RouteSpec{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由配置不能为空",
		)
	}

	gatewayRefs, err := parseGatewayRefs(config.GetGatewayIds())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	hostnames, err := parseHostnames(config.GetHostnames())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	match, err := parseRouteMatch(config.GetMatch())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	accessMode, err := parseAccessMode(config.GetAccessMode())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	upstreamRefs, aiRoute, err := parseForwarding(config.GetForwarding())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	hostRewrite, err := parseHostRewrite(config.GetHostRewrite())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	requestHeaderModifier, err := parseHeaderModifier(config.GetRequestHeaderModifier())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	responseHeaderModifier, err := parseHeaderModifier(config.GetResponseHeaderModifier())
	if err != nil {
		return resource.RouteSpec{}, err
	}
	spec := resource.RouteSpec{
		DisplayName:            displayName,
		Enabled:                config.GetEnabled(),
		AccessMode:             accessMode,
		GatewayRefs:            gatewayRefs,
		Hostnames:              hostnames,
		Match:                  match,
		UpstreamRefs:           upstreamRefs,
		AI:                     aiRoute,
		HostRewrite:            hostRewrite,
		RequestHeaderModifier:  requestHeaderModifier,
		ResponseHeaderModifier: responseHeaderModifier,
		Timeout:                parseTimeout(config.GetTimeout()),
		Retry:                  parseRetry(config.GetRetry()),
	}
	if err := validateRouteSpec(spec); err != nil {
		return resource.RouteSpec{}, err
	}
	return spec, nil
}

func validateRouteSpec(spec resource.RouteSpec) error {
	requestTimeoutMillis := spec.Timeout.RequestMillis
	if requestTimeoutMillis == 0 {
		requestTimeoutMillis = routeconfig.DefaultRequestTimeoutMillis
	} else if requestTimeoutMillis < routeconfig.MinRequestTimeoutMillis ||
		requestTimeoutMillis > routeconfig.MaxRequestTimeoutMillis {
		return errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请求总超时超出允许范围",
		)
	}

	if spec.Retry != nil {
		if spec.Retry.Attempts < routeconfig.MinRetryAttempts ||
			spec.Retry.Attempts > routeconfig.MaxRetryAttempts {
			return errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"重试次数超出允许范围",
			)
		}
		if spec.Retry.PerTryTimeoutMillis < routeconfig.MinPerTryTimeoutMillis ||
			spec.Retry.PerTryTimeoutMillis > routeconfig.MaxPerTryTimeoutMillis {
			return errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"单次重试超时超出允许范围",
			)
		}
		if spec.Retry.PerTryTimeoutMillis > requestTimeoutMillis {
			return errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"单次重试超时不能大于请求总超时",
			)
		}
	}

	if spec.AI == nil {
		return nil
	}
	if len(spec.Match.Methods) != 1 || spec.Match.Methods[0] != http.MethodPost {
		return errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"AI 路由目前只支持 POST 请求",
		)
	}
	if usesReservedAIHeader(spec.Match.Headers, spec.RequestHeaderModifier) {
		return errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"AI 路由不能匹配或修改系统保留的 Header",
		)
	}
	return nil
}

func usesReservedAIHeader(
	matches []resource.HeaderMatch,
	modifier *resource.HeaderModifier,
) bool {
	for _, match := range matches {
		if aiprotocol.IsInternalHeader(match.Name) {
			return true
		}
	}
	if modifier == nil {
		return false
	}
	for _, header := range modifier.Set {
		if aiprotocol.IsInternalHeader(header.Name) {
			return true
		}
	}
	for _, header := range modifier.Add {
		if aiprotocol.IsInternalHeader(header.Name) {
			return true
		}
	}
	for _, name := range modifier.Remove {
		if aiprotocol.IsInternalHeader(name) {
			return true
		}
	}
	return false
}

func parseAccessMode(mode adminv1.RouteAccessMode) (resource.RouteAccessMode, error) {
	switch mode {
	case adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_PUBLIC:
		return resource.RouteAccessPublic, nil
	case adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_CALLER:
		return resource.RouteAccessCaller, nil
	default:
		return "", errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请选择访问方式",
		)
	}
}

func parseHostRewrite(rewrite *adminv1.HostRewrite) (resource.HostRewrite, error) {
	if rewrite == nil {
		return resource.HostRewrite{}, nil
	}

	hostname := strings.TrimSpace(rewrite.GetHostname())
	switch rewrite.GetMode() {
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_HOST:
		if hostname != "" {
			return resource.HostRewrite{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"使用服务端点主机名时不能填写自定义主机名",
			)
		}
		return resource.HostRewrite{Mode: resource.HostRewriteUpstreamHost}, nil
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE:
		if hostname != "" {
			return resource.HostRewrite{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"保留请求主机时不能填写自定义主机名",
			)
		}
		return resource.HostRewrite{Mode: resource.HostRewritePreserve}, nil
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM:
		normalized, ok := hostnameutil.Normalize(hostname)
		if !ok || normalized == "*" {
			return resource.HostRewrite{}, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"自定义主机名格式不正确",
			)
		}
		return resource.HostRewrite{Mode: resource.HostRewriteCustom, Hostname: normalized}, nil
	default:
		return resource.HostRewrite{}, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"请选择转发请求使用的主机名",
		)
	}
}

func parseGatewayRefs(gatewayIDs []string) ([]string, error) {
	if len(gatewayIDs) == 0 {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"至少需要选择一个网关",
		)
	}
	if len(gatewayIDs) > routeconfig.MaxGatewayRefs {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"关联网关数量超过限制",
		)
	}

	gatewayRefs := make([]string, len(gatewayIDs))
	seenGatewayIDs := make(map[string]bool, len(gatewayIDs))
	for i, rawGatewayID := range gatewayIDs {
		gatewayID, valid := resourceconfig.NormalizeID(rawGatewayID)
		if !valid {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"网关 ID 不正确",
			)
		}
		if seenGatewayIDs[gatewayID] {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"同一个网关只能选择一次",
			)
		}
		seenGatewayIDs[gatewayID] = true
		gatewayRefs[i] = gatewayID
	}
	return gatewayRefs, nil
}

func parseHostnames(rawHostnames []string) ([]string, error) {
	if len(rawHostnames) > routeconfig.MaxHostnames {
		return nil, errors.BadRequest(
			adminv1.ErrorReason_INVALID_ARGUMENT.String(),
			"路由域名数量超过限制",
		)
	}
	hostnames := make([]string, len(rawHostnames))
	seenHostnames := make(map[string]bool, len(rawHostnames))
	for i, rawHostname := range rawHostnames {
		hostname, ok := hostnameutil.Normalize(strings.TrimSpace(rawHostname))
		if !ok || hostname == "*" {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"路由域名格式不正确，留空列表表示不限制域名",
			)
		}
		if seenHostnames[hostname] {
			return nil, errors.BadRequest(
				adminv1.ErrorReason_INVALID_ARGUMENT.String(),
				"路由域名不能重复",
			)
		}
		seenHostnames[hostname] = true
		hostnames[i] = hostname
	}
	return hostnames, nil
}

func parseTimeout(timeout *adminv1.RouteTimeout) resource.RouteTimeout {
	if timeout == nil {
		return resource.RouteTimeout{}
	}
	return resource.RouteTimeout{RequestMillis: int(timeout.GetRequestMillis())}
}

func parseRetry(retry *adminv1.RouteRetry) *resource.RouteRetry {
	if retry == nil {
		return nil
	}
	return &resource.RouteRetry{
		Attempts:            int(retry.GetAttempts()),
		PerTryTimeoutMillis: int(retry.GetPerTryTimeoutMillis()),
	}
}
