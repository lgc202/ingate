// Package server 装配 Admin API 的 Kratos transport
package server

import (
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/auth"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/service/authentication"
	"github.com/lgc202/ingate/internal/adminapi/service/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/configuration"
	"github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	"github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/service/route"
	"github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// NewHTTPServer 创建并注册 Admin API 的 Kratos HTTP transport
func NewHTTPServer(
	config *conf.Server,
	logger *slog.Logger,
	authenticator *auth.Authenticator,
	authenticationService *authentication.Service,
	gatewayService *gateway.Service,
	routeService *route.Service,
	upstreamService *upstream.Service,
	certificateService *certificate.Service,
	rateLimitService *ratelimit.Service,
	ipRestrictionService *iprestriction.Service,
	configurationService *configuration.Service,
	healthService *health.Service,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	options := []kratoshttp.ServerOption{
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
		kratoshttp.Filter(requestIDFilter),
		kratoshttp.Middleware(
			recoveryMiddleware(logger),
			requestLoggingMiddleware(logger),
			authenticationMiddleware(authenticator),
			auditMiddleware(logger),
			authorizationMiddleware,
			errorMappingMiddleware,
			requestValidationMiddleware,
		),
		kratoshttp.ResponseEncoder(responseEncoder),
		kratoshttp.ErrorEncoder(errorEncoder),
		kratoshttp.NotFoundHandler(http.HandlerFunc(notFound)),
		kratoshttp.MethodNotAllowedHandler(http.HandlerFunc(methodNotAllowed)),
	}
	httpServer := kratoshttp.NewServer(options...)
	adminv1.RegisterAuthenticationServiceHTTPServer(httpServer, authenticationService)
	adminv1.RegisterGatewayServiceHTTPServer(httpServer, gatewayService)
	adminv1.RegisterRouteServiceHTTPServer(httpServer, routeService)
	adminv1.RegisterUpstreamServiceHTTPServer(httpServer, upstreamService)
	adminv1.RegisterCertificateServiceHTTPServer(httpServer, certificateService)
	adminv1.RegisterRateLimitPolicyServiceHTTPServer(httpServer, rateLimitService)
	adminv1.RegisterIPRestrictionPolicyServiceHTTPServer(httpServer, ipRestrictionService)
	adminv1.RegisterConfigurationServiceHTTPServer(httpServer, configurationService)
	adminv1.RegisterHealthServiceHTTPServer(httpServer, healthService)
	return httpServer
}
