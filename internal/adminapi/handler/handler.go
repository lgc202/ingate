package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	accesscontrolpolicyhandler "github.com/lgc202/ingate/internal/adminapi/handler/accesscontrolpolicy"
	certificatehandler "github.com/lgc202/ingate/internal/adminapi/handler/certificate"
	gatewayhandler "github.com/lgc202/ingate/internal/adminapi/handler/gateway"
	ratelimitpolicyhandler "github.com/lgc202/ingate/internal/adminapi/handler/ratelimitpolicy"
	routehandler "github.com/lgc202/ingate/internal/adminapi/handler/route"
	upstreamhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstream"
	upstreamcredentialhandler "github.com/lgc202/ingate/internal/adminapi/handler/upstreamcredential"
	"github.com/lgc202/ingate/internal/adminapi/pkg/requestid"
	"github.com/lgc202/ingate/internal/adminapi/pkg/response"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// Handler 聚合 admin-api HTTP handler
type Handler struct {
	Certificate         *certificatehandler.Handler
	Gateway             *gatewayhandler.Handler
	Route               *routehandler.Handler
	Upstream            *upstreamhandler.Handler
	UpstreamCredential  *upstreamcredentialhandler.Handler
	AccessControlPolicy *accesscontrolpolicyhandler.Handler
	RateLimitPolicy     *ratelimitpolicyhandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service, logger *slog.Logger) *Handler {
	return &Handler{
		Certificate:         certificatehandler.New(service.Certificate, logger.With("handler", "certificate")),
		Gateway:             gatewayhandler.New(service.Gateway, logger.With("handler", "gateway")),
		Route:               routehandler.New(service.Route, logger.With("handler", "route")),
		Upstream:            upstreamhandler.New(service.Upstream, logger.With("handler", "upstream")),
		UpstreamCredential:  upstreamcredentialhandler.New(service.UpstreamCredential, logger.With("handler", "upstreamcredential")),
		AccessControlPolicy: accesscontrolpolicyhandler.New(service.AccessControlPolicy, logger.With("handler", "accesscontrolpolicy")),
		RateLimitPolicy:     ratelimitpolicyhandler.New(service.RateLimitPolicy, logger.With("handler", "ratelimitpolicy")),
	}
}

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	response.GinJSONResponse(ctx, http.StatusOK, "ok", gin.H{
		"status":    "ok",
		"requestID": ctx.GetString(requestid.Header),
	})
}
