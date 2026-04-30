package handler

import (
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	resourcehandler "github.com/lgc202/ingate/internal/adminapi/handler/resource"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	runtimehandler "github.com/lgc202/ingate/internal/adminapi/handler/runtime"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// Handler 聚合 admin-api HTTP handler
type Handler struct {
	Gateway  *gatewayhandler.Handler
	Resource *resourcehandler.Handler
	Route    *routehandler.Handler
	Runtime  *runtimehandler.Handler
	Upstream *upstreamhandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service) *Handler {
	return &Handler{
		Gateway:  gatewayhandler.New(service.Gateway),
		Resource: resourcehandler.New(service.Resource),
		Route:    routehandler.New(service.Route),
		Runtime:  runtimehandler.New(service.Runtime),
		Upstream: upstreamhandler.New(service.Upstream),
	}
}
