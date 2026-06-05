package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	runtimegrouphandler "github.com/lgc202/ingate/internal/adminapi/handler/runtimegroup"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// Handler 聚合 admin-api HTTP handler
type Handler struct {
	Gateway      *gatewayhandler.Handler
	Route        *routehandler.Handler
	RuntimeGroup *runtimegrouphandler.Handler
	Upstream     *upstreamhandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service, logger *slog.Logger) *Handler {
	return &Handler{
		Gateway:      gatewayhandler.New(service.Gateway, logger.With("handler", "gateway")),
		Route:        routehandler.New(service.Route, logger.With("handler", "route")),
		RuntimeGroup: runtimegrouphandler.New(service.RuntimeGroup, logger.With("handler", "runtimegroup")),
		Upstream:     upstreamhandler.New(service.Upstream, logger.With("handler", "upstream")),
	}
}

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	response.GinJSONResponse(ctx, http.StatusOK, "ok", gin.H{
		"status":    "ok",
		"requestID": ctx.GetString(requestid.Header),
	})
}
