// Package gateway 将已实现的网关、模型和 MCP 资源注册到 Generic API Server 的 API Group。
package gateway

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"

	aibackendstorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/aibackend"
	backendstorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/backend"
	gatewaystorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/gateway"
	routestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/httproute"
	mcproutestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/mcproute"
	modelroutestorage "github.com/lgc202/ingate/internal/apiserver/registry/gateway/modelroute"
)

// NewStorage 创建 gateway.ingate.io API Group 中已实现资源的存储映射。
func NewStorage(
	scheme *runtime.Scheme,
	optionsGetter generic.RESTOptionsGetter,
) (map[string]rest.Storage, error) {
	gatewayREST, statusREST, err := gatewaystorage.NewREST(scheme, optionsGetter)
	if err != nil {
		return nil, fmt.Errorf("create Gateway REST storage: %w", err)
	}

	routeREST, routeStatusREST, err := routestorage.NewREST(scheme, optionsGetter)
	if err != nil {
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create HTTPRoute REST storage: %w", err)
	}

	backendREST, backendStatusREST, err := backendstorage.NewREST(scheme, optionsGetter)
	if err != nil {
		routeREST.Destroy()
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create Backend REST storage: %w", err)
	}

	aiBackendREST, aiBackendStatusREST, err := aibackendstorage.NewREST(scheme, optionsGetter)
	if err != nil {
		backendREST.Destroy()
		routeREST.Destroy()
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create AIBackend REST storage: %w", err)
	}

	modelRouteREST, modelRouteStatusREST, err := modelroutestorage.NewREST(scheme, optionsGetter)
	if err != nil {
		aiBackendREST.Destroy()
		backendREST.Destroy()
		routeREST.Destroy()
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create ModelRoute REST storage: %w", err)
	}

	mcpRouteREST, mcpRouteStatusREST, err := mcproutestorage.NewREST(scheme, optionsGetter)
	if err != nil {
		modelRouteREST.Destroy()
		aiBackendREST.Destroy()
		backendREST.Destroy()
		routeREST.Destroy()
		gatewayREST.Destroy()
		return nil, fmt.Errorf("create MCPRoute REST storage: %w", err)
	}

	return map[string]rest.Storage{
		"gateways":           gatewayREST,
		"gateways/status":    statusREST,
		"httproutes":         routeREST,
		"httproutes/status":  routeStatusREST,
		"backends":           backendREST,
		"backends/status":    backendStatusREST,
		"aibackends":         aiBackendREST,
		"aibackends/status":  aiBackendStatusREST,
		"modelroutes":        modelRouteREST,
		"modelroutes/status": modelRouteStatusREST,
		"mcproutes":          mcpRouteREST,
		"mcproutes/status":   mcpRouteStatusREST,
	}, nil
}
