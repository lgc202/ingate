package compiler

import (
	"fmt"
	"maps"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	extauthzv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	gatewayv1 "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/extauthz"
)

const (
	authzClusterName    = "ingate-system-authz"
	authzRequestTimeout = 2 * time.Second
)

// buildAuthorizationHTTPFilter 使用 Envoy 官方 ext_authz 过滤器连接同步准入服务。
// 过滤器默认关闭，只由需要 Caller 身份或限流规则的 Route 显式启用。
func buildAuthorizationHTTPFilter() (*hcmv3.HttpFilter, error) {
	configuration := &extauthzv3.ExtAuthz{
		Services: &extauthzv3.ExtAuthz_GrpcService{GrpcService: &corev3.GrpcService{
			TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{ClusterName: authzClusterName},
			},
			Timeout: durationpb.New(authzRequestTimeout),
		}},
		TransportApiVersion: corev3.ApiVersion_V3,
		FailureModeAllow:    false,
		StatusOnError:       &typev3.HttpStatus{Code: typev3.StatusCode_ServiceUnavailable},
		ValidateMutations:   true,
		// gRPC ext_authz 未配置 AllowedHeaders 时会按 Envoy 协议传递全部请求 Header。
		// Header 主体限流依赖该标准行为，Authz 只在进程内读取这些值，不会写入日志。
	}
	if err := configuration.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate authorization filter: %w", err)
	}
	typedConfig, err := anypb.New(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode authorization filter: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name:       extauthz.FilterName,
		Disabled:   true,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: typedConfig},
	}, nil
}

func (c *compilation) routeAccessConfig(route *gatewayv1.Route) (*anypb.Any, bool) {
	switch route.Spec.AccessMode {
	case gatewayv1.RouteAccessPublic:
		return nil, true
	case gatewayv1.RouteAccessCaller:
		typedConfig, err := newAuthzPerRouteConfig(map[string]string{
			extauthz.RouteIDContext:        route.Name,
			extauthz.CallerRequiredContext: "true",
		})
		if err != nil {
			c.addRouteError(
				route.Name,
				ReasonCompileFailed,
				fmt.Sprintf("encode Caller authorization for route %q: %v", route.Name, err),
			)
			return nil, false
		}
		return typedConfig, true
	default:
		c.addRouteError(
			route.Name,
			ReasonUnsupported,
			fmt.Sprintf("route %q uses unsupported access mode %q", route.Name, route.Spec.AccessMode),
		)
		return nil, false
	}
}

func applyAuthzContext(routes []*routev3.Route, extensions map[string]string) error {
	for _, route := range routes {
		merged := maps.Clone(extensions)
		if current := route.GetTypedPerFilterConfig()[extauthz.FilterName]; current != nil {
			configuration := new(extauthzv3.ExtAuthzPerRoute)
			if err := current.UnmarshalTo(configuration); err != nil {
				return fmt.Errorf("decode existing route authorization config: %w", err)
			}
			maps.Copy(merged, configuration.GetCheckSettings().GetContextExtensions())
		}
		typedConfig, err := newAuthzPerRouteConfig(merged)
		if err != nil {
			return err
		}
		if route.TypedPerFilterConfig == nil {
			route.TypedPerFilterConfig = make(map[string]*anypb.Any)
		}
		route.TypedPerFilterConfig[extauthz.FilterName] = typedConfig
	}
	return nil
}

func newAuthzPerRouteConfig(extensions map[string]string) (*anypb.Any, error) {
	configuration := &extauthzv3.ExtAuthzPerRoute{
		Override: &extauthzv3.ExtAuthzPerRoute_CheckSettings{CheckSettings: &extauthzv3.CheckSettings{
			ContextExtensions: extensions,
		}},
	}
	typedConfig, err := anypb.New(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode route authorization config: %w", err)
	}
	return typedConfig, nil
}
