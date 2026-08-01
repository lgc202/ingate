package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	accesscontrolpolicyhandler "github.com/lgc202/ingate/internal/admin/handler/accesscontrolpolicy"
	certificatehandler "github.com/lgc202/ingate/internal/admin/handler/certificate"
	configurationstatushandler "github.com/lgc202/ingate/internal/admin/handler/configurationstatus"
	gatewayhandler "github.com/lgc202/ingate/internal/admin/handler/gateway"
	ratelimitpolicyhandler "github.com/lgc202/ingate/internal/admin/handler/ratelimitpolicy"
	routehandler "github.com/lgc202/ingate/internal/admin/handler/route"
	tokenquotapolicyhandler "github.com/lgc202/ingate/internal/admin/handler/tokenquotapolicy"
	upstreamhandler "github.com/lgc202/ingate/internal/admin/handler/upstream"
	"github.com/lgc202/ingate/internal/admin/pkg/response"
	"github.com/lgc202/ingate/internal/admin/service"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// Handler 聚合管理 API 的 HTTP 入口
type Handler struct {
	Certificate         *certificatehandler.Handler
	ConfigurationStatus *configurationstatushandler.Handler
	Gateway             *gatewayhandler.Handler
	Route               *routehandler.Handler
	Upstream            *upstreamhandler.Handler
	AccessControlPolicy *accesscontrolpolicyhandler.Handler
	RateLimitPolicy     *ratelimitpolicyhandler.Handler
	TokenQuotaPolicy    *tokenquotapolicyhandler.Handler
}

// New 创建 handler 聚合入口
func New(service *service.Service, logger *slog.Logger) *Handler {
	return &Handler{
		Certificate:         certificatehandler.New(service.Certificate, logger.With("handler", "certificate")),
		ConfigurationStatus: configurationstatushandler.New(service.ConfigurationStatus, logger.With("handler", "configurationstatus")),
		Gateway:             gatewayhandler.New(service.Gateway, logger.With("handler", "gateway")),
		Route:               routehandler.New(service.Route, logger.With("handler", "route")),
		Upstream:            upstreamhandler.New(service.Upstream, logger.With("handler", "upstream")),
		AccessControlPolicy: accesscontrolpolicyhandler.New(service.AccessControlPolicy, logger.With("handler", "accesscontrolpolicy")),
		RateLimitPolicy:     ratelimitpolicyhandler.New(service.RateLimitPolicy, logger.With("handler", "ratelimitpolicy")),
		TokenQuotaPolicy:    tokenquotapolicyhandler.New(service.TokenQuotaPolicy, logger.With("handler", "tokenquotapolicy")),
	}
}

// Health 返回服务健康状态
func (h *Handler) Health(ctx *gin.Context) {
	response.GinJSONResponse(ctx, http.StatusOK, "ok", gin.H{
		"status":    "ok",
		"requestID": ctx.GetString(requestid.Header),
	})
}
