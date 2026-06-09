package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	accesscontrolpolicyhandler "github.com/lgc202/ingate/internal/adminapi/handler/accesscontrolpolicy"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	policybindinghandler "github.com/lgc202/ingate/internal/adminapi/handler/policybinding"
	ratelimitpolicyhandler "github.com/lgc202/ingate/internal/adminapi/handler/ratelimitpolicy"
	redisstorehandler "github.com/lgc202/ingate/internal/adminapi/handler/redisstore"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	runtimegrouphandler "github.com/lgc202/ingate/internal/adminapi/handler/runtimegroup"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// Handler 聚合 admin-api HTTP handler
type Handler struct {
	Gateway             *gatewayhandler.Handler
	Route               *routehandler.Handler
	RuntimeGroup        *runtimegrouphandler.Handler
	Upstream            *upstreamhandler.Handler
	AccessControlPolicy *accesscontrolpolicyhandler.Handler
	RateLimitPolicy     *ratelimitpolicyhandler.Handler
	PolicyBinding       *policybindinghandler.Handler
	RedisStore          *redisstorehandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service, logger *slog.Logger) *Handler {
	return &Handler{
		Gateway:             gatewayhandler.New(service.Gateway, logger.With("handler", "gateway")),
		Route:               routehandler.New(service.Route, logger.With("handler", "route")),
		RuntimeGroup:        runtimegrouphandler.New(service.RuntimeGroup, logger.With("handler", "runtimegroup")),
		Upstream:            upstreamhandler.New(service.Upstream, logger.With("handler", "upstream")),
		AccessControlPolicy: accesscontrolpolicyhandler.New(service.AccessControlPolicy, logger.With("handler", "accesscontrolpolicy")),
		RateLimitPolicy:     ratelimitpolicyhandler.New(service.RateLimitPolicy, logger.With("handler", "ratelimitpolicy")),
		PolicyBinding:       policybindinghandler.New(service.PolicyBinding, logger.With("handler", "policybinding")),
		RedisStore:          redisstorehandler.New(service.RedisStore, logger.With("handler", "redisstore")),
	}
}

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	response.GinJSONResponse(ctx, http.StatusOK, "ok", gin.H{
		"status":    "ok",
		"requestID": ctx.GetString(requestid.Header),
	})
}
