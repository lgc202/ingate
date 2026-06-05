package handler

import (
	"github.com/gin-gonic/gin"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
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

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	response.WriteResult(ctx, gin.H{
		"status":    "ok",
		"requestID": ctx.GetString(requestid.Header),
	}, nil)
}
