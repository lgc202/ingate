package adminapi

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"golang.org/x/sync/errgroup"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	agenttool "github.com/lgc202/ingate/internal/assistant/biz/agent/tool"
)

const resourceLookupConcurrency = 8

// ListRoutes 查询当前配置域中的路由。
func (c *Client) ListRoutes(ctx context.Context, query agenttool.ResourceListQuery) (agenttool.RoutePage, error) {
	result, err := c.routes.ListRoutes(ctx, &adminv1.ListRoutesRequest{
		Query: query.Text,
		Limit: query.Limit,
	})
	if err != nil {
		return agenttool.RoutePage{}, fmt.Errorf("list routes from Admin API: %w", err)
	}
	if result == nil {
		return agenttool.RoutePage{}, errors.New("list routes from Admin API: empty response")
	}

	routes := make([]agenttool.Route, 0, len(result.GetRoutes()))
	for _, route := range result.GetRoutes() {
		if err := validateRouteResponse(route); err != nil {
			return agenttool.RoutePage{}, err
		}
		routes = append(routes, routeFromAPI(route))
	}
	return agenttool.RoutePage{
		Items:   routes,
		HasMore: result.GetNextCursor() != "",
	}, nil
}

// GetRouteConfiguration 读取一条路由及其直接引用的网关和服务。
// 这里按引用 ID 精确查询，既不会扫描全部资源，
// 也不会把 Admin API 客户端暴露给 Agent 业务层。
func (c *Client) GetRouteConfiguration(
	ctx context.Context,
	routeID string,
) (agenttool.RouteConfiguration, error) {
	route, err := c.routes.GetRoute(ctx, &adminv1.GetRouteRequest{Id: routeID})
	if err != nil {
		return agenttool.RouteConfiguration{}, queryTargetError(
			fmt.Sprintf("get route %s from Admin API", routeID),
			err,
		)
	}
	if err := validateRouteResponse(route); err != nil {
		return agenttool.RouteConfiguration{}, err
	}
	config := route.GetConfig()

	gatewayIDs := slices.Clone(config.GetGatewayIds())
	serviceIDs := routeServiceIDs(route)
	gateways := make([]agenttool.Gateway, len(gatewayIDs))
	services := make([]agenttool.Service, len(serviceIDs))
	group, lookupCtx := errgroup.WithContext(ctx)
	group.SetLimit(resourceLookupConcurrency)
	for index, gatewayID := range gatewayIDs {
		group.Go(func() error {
			gateway, err := c.gateways.GetGateway(
				lookupCtx,
				&adminv1.GetGatewayRequest{Id: gatewayID},
			)
			if err != nil {
				return queryTargetError(
					fmt.Sprintf("get gateway %s referenced by route %s", gatewayID, routeID),
					err,
				)
			}
			if err := validateGatewayResponse(gateway); err != nil {
				return err
			}
			gateways[index] = gatewayFromAPI(gateway)
			return nil
		})
	}
	for index, serviceID := range serviceIDs {
		group.Go(func() error {
			service, err := c.services.GetUpstream(
				lookupCtx,
				&adminv1.GetUpstreamRequest{Id: serviceID},
			)
			if err != nil {
				return queryTargetError(
					fmt.Sprintf("get service %s referenced by route %s", serviceID, routeID),
					err,
				)
			}
			if err := validateServiceResponse(service); err != nil {
				return err
			}
			services[index] = serviceFromAPI(service)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return agenttool.RouteConfiguration{}, err
	}

	return agenttool.RouteConfiguration{
		Route:             routeFromAPI(route),
		Hostnames:         slices.Clone(config.GetHostnames()),
		PathMatchType:     routePathMatchType(config.GetMatch().GetPath().GetType()),
		Methods:           routeMethods(config.GetMatch().GetMethods()),
		Targets:           routeTargets(route),
		RequestTimeout:    milliseconds(config.GetTimeout().GetRequestMillis()),
		RetryAttempts:     config.GetRetry().GetAttempts(),
		PerTryTimeout:     milliseconds(config.GetRetry().GetPerTryTimeoutMillis()),
		HostRewriteMode:   hostRewriteMode(config.GetHostRewrite().GetMode()),
		HostRewriteTarget: config.GetHostRewrite().GetHostname(),
		Gateways:          gateways,
		Services:          services,
	}, nil
}

func routeFromAPI(route *adminv1.Route) agenttool.Route {
	config := route.GetConfig()
	return agenttool.Route{
		ID:            route.GetId(),
		Name:          route.GetName(),
		Type:          routeType(route),
		Enabled:       config.GetEnabled(),
		State:         resourceState(route.GetState()),
		Message:       route.GetMessage(),
		AccessMode:    routeAccessMode(config.GetAccessMode()),
		GatewayIDs:    slices.Clone(config.GetGatewayIds()),
		Path:          config.GetMatch().GetPath().GetValue(),
		ServiceIDs:    routeServiceIDs(route),
		ExposedModels: routeModelNames(route),
	}
}

func routeType(route *adminv1.Route) string {
	if route.GetConfig().GetForwarding().GetAi() != nil {
		return "ai"
	}
	return "api"
}

func routeAccessMode(mode adminv1.RouteAccessMode) string {
	switch mode {
	case adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_PUBLIC:
		return "public"
	case adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_CALLER:
		return "caller"
	default:
		return "unknown"
	}
}

func routePathMatchType(matchType adminv1.RoutePathMatchType) string {
	switch matchType {
	case adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_EXACT:
		return "exact"
	default:
		return "prefix"
	}
}

func routeMethods(methods []adminv1.HTTPMethod) []string {
	result := make([]string, 0, len(methods))
	for _, method := range methods {
		if name := httpMethod(method); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func httpMethod(method adminv1.HTTPMethod) string {
	switch method {
	case adminv1.HTTPMethod_HTTP_METHOD_GET:
		return "GET"
	case adminv1.HTTPMethod_HTTP_METHOD_HEAD:
		return "HEAD"
	case adminv1.HTTPMethod_HTTP_METHOD_POST:
		return "POST"
	case adminv1.HTTPMethod_HTTP_METHOD_PUT:
		return "PUT"
	case adminv1.HTTPMethod_HTTP_METHOD_PATCH:
		return "PATCH"
	case adminv1.HTTPMethod_HTTP_METHOD_DELETE:
		return "DELETE"
	case adminv1.HTTPMethod_HTTP_METHOD_OPTIONS:
		return "OPTIONS"
	default:
		return ""
	}
}

// routeTargets 同时保留客户端模型名和厂商模型名，便于判断 AI 路由是否选错线路。
// 普通 API 路由没有模型映射，只返回服务和权重。
func routeTargets(route *adminv1.Route) []agenttool.RouteTarget {
	forwarding := route.GetConfig().GetForwarding()
	if forwarding.GetAi() == nil {
		services := forwarding.GetService().GetTargets()
		targets := make([]agenttool.RouteTarget, 0, len(services))
		for _, target := range services {
			targets = append(targets, agenttool.RouteTarget{
				ServiceID: target.GetServiceId(),
				Weight:    target.GetWeight(),
			})
		}
		return targets
	}

	var count int
	for _, model := range forwarding.GetAi().GetModels() {
		count += len(model.GetTargets())
	}
	targets := make([]agenttool.RouteTarget, 0, count)
	for _, model := range forwarding.GetAi().GetModels() {
		for _, target := range model.GetTargets() {
			targets = append(targets, agenttool.RouteTarget{
				ServiceID:    target.GetServiceId(),
				ExposedModel: model.GetName(),
				Model:        target.GetModel(),
				Weight:       target.GetWeight(),
			})
		}
	}
	return targets
}

func hostRewriteMode(mode adminv1.HostRewriteMode) string {
	switch mode {
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_SERVICE_HOST:
		return "service_host"
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_PRESERVE:
		return "preserve"
	case adminv1.HostRewriteMode_HOST_REWRITE_MODE_CUSTOM:
		return "custom"
	default:
		return ""
	}
}

func routeServiceIDs(route *adminv1.Route) []string {
	forwarding := route.GetConfig().GetForwarding()
	services := forwarding.GetService().GetTargets()
	serviceIDs := make([]string, 0, len(services))
	seen := make(map[string]bool, len(services))
	for _, service := range services {
		serviceID := service.GetServiceId()
		if !seen[serviceID] {
			seen[serviceID] = true
			serviceIDs = append(serviceIDs, serviceID)
		}
	}
	for _, model := range forwarding.GetAi().GetModels() {
		for _, target := range model.GetTargets() {
			serviceID := target.GetServiceId()
			if !seen[serviceID] {
				seen[serviceID] = true
				serviceIDs = append(serviceIDs, serviceID)
			}
		}
	}
	return serviceIDs
}

func routeModelNames(route *adminv1.Route) []string {
	models := route.GetConfig().GetForwarding().GetAi().GetModels()
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.GetName())
	}
	return names
}

func validateRouteResponse(route *adminv1.Route) error {
	if route == nil || !validResourceID(route.GetId()) || route.GetName() == "" {
		return errors.New("invalid route returned by Admin API")
	}
	if !validResourceState(route.GetState()) {
		return fmt.Errorf("route %s returned an invalid resource state", route.GetId())
	}
	config := route.GetConfig()
	if config == nil || config.Enabled == nil || len(config.GetGatewayIds()) == 0 ||
		config.GetMatch() == nil || config.GetMatch().GetPath() == nil ||
		config.GetForwarding() == nil {
		return fmt.Errorf("route %s returned an incomplete configuration", route.GetId())
	}
	if config.GetAccessMode() != adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_PUBLIC &&
		config.GetAccessMode() != adminv1.RouteAccessMode_ROUTE_ACCESS_MODE_CALLER {
		return fmt.Errorf("route %s returned an invalid access mode", route.GetId())
	}
	pathType := config.GetMatch().GetPath().GetType()
	if pathType != adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_PREFIX &&
		pathType != adminv1.RoutePathMatchType_ROUTE_PATH_MATCH_TYPE_EXACT {
		return fmt.Errorf("route %s returned an invalid path match type", route.GetId())
	}
	forwarding := config.GetForwarding()
	if (forwarding.GetService() == nil) == (forwarding.GetAi() == nil) {
		return fmt.Errorf("route %s returned an invalid forwarding configuration", route.GetId())
	}
	for _, method := range config.GetMatch().GetMethods() {
		if httpMethod(method) == "" {
			return fmt.Errorf("route %s returned an invalid HTTP method", route.GetId())
		}
	}
	for _, gatewayID := range config.GetGatewayIds() {
		if !validResourceID(gatewayID) {
			return fmt.Errorf("route %s returned an invalid gateway reference", route.GetId())
		}
	}
	for _, serviceID := range routeServiceIDs(route) {
		if !validResourceID(serviceID) {
			return fmt.Errorf("route %s returned an invalid service reference", route.GetId())
		}
	}
	if serviceForwarding := forwarding.GetService(); serviceForwarding != nil {
		if len(serviceForwarding.GetTargets()) == 0 {
			return fmt.Errorf("route %s returned no service targets", route.GetId())
		}
		for _, target := range serviceForwarding.GetTargets() {
			if target == nil || !validResourceID(target.GetServiceId()) || target.GetWeight() == 0 {
				return fmt.Errorf("route %s returned an invalid service target", route.GetId())
			}
		}
	}
	if aiForwarding := forwarding.GetAi(); aiForwarding != nil {
		if len(aiForwarding.GetModels()) == 0 {
			return fmt.Errorf("route %s returned no AI model mappings", route.GetId())
		}
		for _, model := range aiForwarding.GetModels() {
			if model == nil || model.GetName() == "" || len(model.GetTargets()) == 0 {
				return fmt.Errorf("route %s returned an invalid AI model mapping", route.GetId())
			}
			for _, target := range model.GetTargets() {
				if target == nil || !validResourceID(target.GetServiceId()) ||
					target.GetModel() == "" || target.GetWeight() == 0 {
					return fmt.Errorf("route %s returned an invalid AI model target", route.GetId())
				}
			}
		}
	}
	return nil
}
