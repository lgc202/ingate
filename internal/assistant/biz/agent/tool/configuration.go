package tool

import (
	"context"
	"fmt"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/google/uuid"
)

type routeConfigurationInput struct {
	RouteID string `json:"route_id" jsonschema_description:"要检查的路由 ID；使用 analyze_traffic 或 list_routes 返回的 ID"`
}

type routeConfigurationOutput struct {
	Summary  string                  `json:"summary"`
	Source   string                  `json:"source,omitempty"`
	Status   string                  `json:"status"`
	Route    *routeConfigurationInfo `json:"route,omitempty"`
	Gateways []gatewayInfo           `json:"gateways,omitempty"`
	Services []serviceInfo           `json:"services,omitempty"`
}

type routeConfigurationInfo struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Type          string                `json:"type"`
	Enabled       bool                  `json:"enabled"`
	State         string                `json:"state"`
	Message       string                `json:"message,omitempty"`
	AccessMode    string                `json:"access_mode"`
	Hostnames     []string              `json:"hostnames,omitempty"`
	Match         routeMatchInfo        `json:"match"`
	Targets       []routeTargetInfo     `json:"targets"`
	Timeout       *routeTimeoutInfo     `json:"timeout,omitempty"`
	Retry         *routeRetryInfo       `json:"retry,omitempty"`
	HostRewrite   *routeHostRewriteInfo `json:"host_rewrite,omitempty"`
	ExposedModels []string              `json:"exposed_models,omitempty"`
}

type routeMatchInfo struct {
	Type    string   `json:"type"`
	Path    string   `json:"path"`
	Methods []string `json:"methods,omitempty"`
}

type routeTargetInfo struct {
	ServiceID    string `json:"service_id"`
	ServiceName  string `json:"service_name"`
	ExposedModel string `json:"exposed_model,omitempty"`
	Model        string `json:"model,omitempty"`
	Weight       uint32 `json:"weight"`
}

type routeTimeoutInfo struct {
	RequestMillis int64 `json:"request_millis,omitempty"`
}

type routeRetryInfo struct {
	Attempts            uint32 `json:"attempts,omitempty"`
	PerTryTimeoutMillis int64  `json:"per_try_timeout_millis,omitempty"`
}

type routeHostRewriteInfo struct {
	Mode     string `json:"mode,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

func newRouteConfigurationTool(resources RouteConfigurationReader) (einotool.BaseTool, error) {
	definition, err := utils.InferTool(
		getRouteConfigTool,
		"查询一条路由及其关联网关、目标服务的完整生效关系。用于流量异常后的配置核查，不用于流量排名。",
		func(ctx context.Context, input routeConfigurationInput) (routeConfigurationOutput, error) {
			return getRouteConfiguration(ctx, resources, input)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("define %s tool: %w", getRouteConfigTool, err)
	}
	return definition, nil
}

func getRouteConfiguration(
	ctx context.Context,
	resources RouteConfigurationReader,
	input routeConfigurationInput,
) (routeConfigurationOutput, error) {
	routeID := strings.TrimSpace(input.RouteID)
	if _, err := uuid.Parse(routeID); err != nil {
		return routeConfigurationInputResult(
			invalidInputf("route_id must be a valid route ID returned by analyze_traffic or list_routes"),
		)
	}

	configuration, err := resources.GetRouteConfiguration(ctx, routeID)
	if err != nil {
		return routeConfigurationOutput{}, err
	}
	serviceNames := make(map[string]string, len(configuration.Services))
	services := make([]serviceInfo, 0, len(configuration.Services))
	for _, service := range configuration.Services {
		serviceNames[service.ID] = service.Name
		services = append(services, serviceInfoFromResource(service))
	}
	targets := make([]routeTargetInfo, 0, len(configuration.Targets))
	for _, target := range configuration.Targets {
		targets = append(targets, routeTargetInfo{
			ServiceID:    target.ServiceID,
			ServiceName:  serviceNames[target.ServiceID],
			ExposedModel: target.ExposedModel,
			Model:        target.Model,
			Weight:       target.Weight,
		})
	}
	gateways := make([]gatewayInfo, 0, len(configuration.Gateways))
	for _, gateway := range configuration.Gateways {
		gateways = append(gateways, gatewayInfoFromResource(gateway))
	}
	timeout := routeTimeout(configuration.RequestTimeout.Milliseconds())
	retry := routeRetry(
		configuration.RetryAttempts,
		configuration.PerTryTimeout.Milliseconds(),
	)
	hostRewrite := routeHostRewrite(
		configuration.HostRewriteMode,
		configuration.HostRewriteTarget,
	)

	route := configuration.Route
	return routeConfigurationOutput{
		Summary: fmt.Sprintf(
			"已解析路由 %s 关联的 %d 个网关和 %d 个目标服务",
			route.Name,
			len(gateways),
			len(services),
		),
		Source: "admin_api",
		Status: "complete",
		Route: &routeConfigurationInfo{
			ID:         route.ID,
			Name:       route.Name,
			Type:       route.Type,
			Enabled:    route.Enabled,
			State:      route.State,
			Message:    route.Message,
			AccessMode: route.AccessMode,
			Hostnames:  configuration.Hostnames,
			Match: routeMatchInfo{
				Type:    configuration.PathMatchType,
				Path:    route.Path,
				Methods: configuration.Methods,
			},
			Targets:       targets,
			Timeout:       timeout,
			Retry:         retry,
			HostRewrite:   hostRewrite,
			ExposedModels: route.ExposedModels,
		},
		Gateways: gateways,
		Services: services,
	}, nil
}

func routeTimeout(requestMillis int64) *routeTimeoutInfo {
	if requestMillis <= 0 {
		return nil
	}
	return &routeTimeoutInfo{RequestMillis: requestMillis}
}

func routeRetry(attempts uint32, perTryTimeoutMillis int64) *routeRetryInfo {
	if attempts == 0 && perTryTimeoutMillis <= 0 {
		return nil
	}
	return &routeRetryInfo{
		Attempts:            attempts,
		PerTryTimeoutMillis: perTryTimeoutMillis,
	}
}

func routeHostRewrite(mode, hostname string) *routeHostRewriteInfo {
	if mode == "" && hostname == "" {
		return nil
	}
	return &routeHostRewriteInfo{Mode: mode, Hostname: hostname}
}

func routeConfigurationInputResult(err error) (routeConfigurationOutput, error) {
	reason, ok := invalidInputReason(err)
	if !ok {
		return routeConfigurationOutput{}, err
	}
	return routeConfigurationOutput{
		Summary: reason,
		Status:  "invalid_input",
	}, nil
}
