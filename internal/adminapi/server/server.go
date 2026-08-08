// Package server 装配 Admin API 的 Kratos transport
package server

import (
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
	"github.com/lgc202/ingate/internal/adminapi/service"
)

// ProviderSet 汇总 Admin API transport
var ProviderSet = wire.NewSet(NewHTTPServer)

// NewHTTPServer 创建并注册 Admin API 的 Kratos HTTP transport
func NewHTTPServer(
	config *conf.Server,
	logger *slog.Logger,
	gateway *service.GatewayService,
	route *service.RouteService,
	upstream *service.UpstreamService,
	certificate *service.CertificateService,
	accessKey *service.AccessKeyService,
	rateLimit *service.RateLimitPolicyService,
	accessControl *service.AccessControlPolicyService,
	tokenQuota *service.TokenQuotaPolicyService,
	configuration *service.ConfigurationService,
	health *service.HealthService,
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
			requestValidationMiddleware,
		),
		kratoshttp.ResponseEncoder(responseEncoder),
		kratoshttp.ErrorEncoder(errorEncoder),
		kratoshttp.NotFoundHandler(http.HandlerFunc(notFound)),
		kratoshttp.MethodNotAllowedHandler(http.HandlerFunc(methodNotAllowed)),
	}
	httpServer := kratoshttp.NewServer(options...)
	adminv1.RegisterGatewayServiceHTTPServer(httpServer, gateway)
	adminv1.RegisterRouteServiceHTTPServer(httpServer, route)
	adminv1.RegisterUpstreamServiceHTTPServer(httpServer, upstream)
	adminv1.RegisterCertificateServiceHTTPServer(httpServer, certificate)
	adminv1.RegisterAccessKeyServiceHTTPServer(httpServer, accessKey)
	adminv1.RegisterRateLimitPolicyServiceHTTPServer(httpServer, rateLimit)
	adminv1.RegisterAccessControlPolicyServiceHTTPServer(httpServer, accessControl)
	adminv1.RegisterTokenQuotaPolicyServiceHTTPServer(httpServer, tokenQuota)
	adminv1.RegisterConfigurationServiceHTTPServer(httpServer, configuration)
	adminv1.RegisterHealthServiceHTTPServer(httpServer, health)
	return httpServer
}
