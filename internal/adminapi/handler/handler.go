package handler

import (
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// Handler 聚合 admin-api HTTP handler
type Handler struct {
	Gateway  *gatewayhandler.Handler
	Route    *routehandler.Handler
	Upstream *upstreamhandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service) *Handler {
	return &Handler{
		Gateway:  gatewayhandler.New(service.Gateway),
		Route:    routehandler.New(service.Route),
		Upstream: upstreamhandler.New(service.Upstream),
	}
}
