package compiler

import (
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extauthzv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/lgc202/ingate/internal/authz/filterconfig"
	gatewayv1 "github.com/lgc202/ingate/pkg/apis/gateway/v1"
)

const (
	authzClusterName    = "ingate-system-authz"
	authzRequestTimeout = 2 * time.Second
)

// buildCallerAuthHTTPFilter 使用 Envoy 官方 ext_authz 过滤器连接 Caller 鉴权服务
// 过滤器默认关闭，只由 AccessMode=Caller 的 Route 显式启用
func buildCallerAuthHTTPFilter() (*hcmv3.HttpFilter, error) {
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
		AllowedHeaders: &matcherv3.ListStringMatcher{Patterns: []*matcherv3.StringMatcher{{
			MatchPattern: &matcherv3.StringMatcher_Exact{Exact: "authorization"},
			IgnoreCase:   true,
		}}},
	}
	if err := configuration.ValidateAll(); err != nil {
		return nil, fmt.Errorf("validate Caller authorization filter: %w", err)
	}
	typedConfig, err := anypb.New(configuration)
	if err != nil {
		return nil, fmt.Errorf("encode Caller authorization filter: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name:       filterconfig.HTTPFilterName,
		Disabled:   true,
		ConfigType: &hcmv3.HttpFilter_TypedConfig{TypedConfig: typedConfig},
	}, nil
}

func (c *compilation) routeAccessConfig(route *gatewayv1.Route) (*anypb.Any, bool) {
	switch route.Spec.AccessMode {
	case gatewayv1.RouteAccessPublic:
		return nil, true
	case gatewayv1.RouteAccessCaller:
		configuration := &extauthzv3.ExtAuthzPerRoute{
			Override: &extauthzv3.ExtAuthzPerRoute_CheckSettings{CheckSettings: &extauthzv3.CheckSettings{
				ContextExtensions: map[string]string{filterconfig.RouteIDContext: route.Name},
			}},
		}
		typedConfig, err := anypb.New(configuration)
		if err != nil {
			c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonCompileFailed, fmt.Sprintf("encode Caller authorization for route %q: %v", route.Name, err))
			return nil, false
		}
		return typedConfig, true
	default:
		c.addDiagnostic(SeverityError, gatewayv1.KindRoute, route.Name, ReasonUnsupported, fmt.Sprintf("route %q uses unsupported access mode %q", route.Name, route.Spec.AccessMode))
		return nil, false
	}
}
