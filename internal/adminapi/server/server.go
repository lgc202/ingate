// Package server 装配 Admin API 的 Kratos transport
package server

import (
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/service/accesscontrol"
	"github.com/lgc202/ingate/internal/adminapi/service/accesskey"
	"github.com/lgc202/ingate/internal/adminapi/service/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/configuration"
	"github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	"github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/service/route"
	"github.com/lgc202/ingate/internal/adminapi/service/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// ProviderSet 汇总 Admin API transport
var ProviderSet = wire.NewSet(NewHTTPServer)

// NewHTTPServer 创建并注册 Admin API 的 Kratos HTTP transport
func NewHTTPServer(
	config *conf.Server,
	logger *slog.Logger,
	gatewayService *gateway.Service,
	routeService *route.Service,
	upstreamService *upstream.Service,
	certificateService *certificate.Service,
	accessKeyService *accesskey.Service,
	rateLimitService *ratelimit.Service,
	accessControlService *accesscontrol.Service,
	tokenQuotaService *tokenquota.Service,
	configurationService *configuration.Service,
	healthService *health.Service,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	options := []kratoshttp.ServerOption{
		kratoshttp.Network(httpConfig.GetNetwork()),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
		kratoshttp.Filter(requestIDFilter),
		kratoshttp.Middleware(
			recoveryMiddleware(logger),
			requestLoggingMiddleware(logger),
			errorMappingMiddleware,
			requestValidationMiddleware,
		),
		kratoshttp.ResponseEncoder(responseEncoder),
		kratoshttp.ErrorEncoder(errorEncoder),
		kratoshttp.NotFoundHandler(http.HandlerFunc(notFound)),
		kratoshttp.MethodNotAllowedHandler(http.HandlerFunc(methodNotAllowed)),
	}
	httpServer := kratoshttp.NewServer(options...)
	adminv1.RegisterGatewayServiceHTTPServer(httpServer, gatewayService)
	adminv1.RegisterRouteServiceHTTPServer(httpServer, routeService)
	adminv1.RegisterUpstreamServiceHTTPServer(httpServer, upstreamService)
	adminv1.RegisterCertificateServiceHTTPServer(httpServer, certificateService)
	adminv1.RegisterAccessKeyServiceHTTPServer(httpServer, accessKeyService)
	adminv1.RegisterRateLimitPolicyServiceHTTPServer(httpServer, rateLimitService)
	adminv1.RegisterAccessControlPolicyServiceHTTPServer(httpServer, accessControlService)
	adminv1.RegisterTokenQuotaPolicyServiceHTTPServer(httpServer, tokenQuotaService)
	adminv1.RegisterConfigurationServiceHTTPServer(httpServer, configurationService)
	adminv1.RegisterHealthServiceHTTPServer(httpServer, healthService)
	return httpServer
}
