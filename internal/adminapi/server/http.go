package server

import (
	"log/slog"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/conf"
)

// NewHTTPServer 创建 Admin API 的 Kratos HTTP transport。
func NewHTTPServer(
	config *conf.Server,
	logger *slog.Logger,
	services *Services,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	httpServer := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
		kratoshttp.Filter(requestIDFilter, requestBodyLimitFilter),
		kratoshttp.Middleware(serverMiddleware(logger)...),
		kratoshttp.RequestVarsDecoder(requestVarsDecoder),
		kratoshttp.RequestQueryDecoder(requestQueryDecoder),
		kratoshttp.RequestDecoder(requestDecoder),
		kratoshttp.ResponseEncoder(responseEncoder),
		kratoshttp.ErrorEncoder(errorEncoder),
		kratoshttp.NotFoundHandler(http.HandlerFunc(endpointNotFoundHandler)),
		kratoshttp.MethodNotAllowedHandler(http.HandlerFunc(methodNotAllowedHandler)),
	)
	services.registerHTTP(httpServer)
	return httpServer
}

func (s *Services) registerHTTP(server *kratoshttp.Server) {
	adminv1.RegisterAIUsageServiceHTTPServer(server, s.aiUsage)
	adminv1.RegisterCallerServiceHTTPServer(server, s.caller)
	adminv1.RegisterGatewayServiceHTTPServer(server, s.gateway)
	adminv1.RegisterRouteServiceHTTPServer(server, s.route)
	adminv1.RegisterServiceManagementServiceHTTPServer(server, s.service)
	adminv1.RegisterCertificateServiceHTTPServer(server, s.certificate)
	adminv1.RegisterRateLimitPolicyServiceHTTPServer(server, s.rateLimit)
	adminv1.RegisterIPRestrictionPolicyServiceHTTPServer(server, s.ipRestriction)
	adminv1.RegisterRequestRecordServiceHTTPServer(server, s.request)
	adminv1.RegisterTrafficAnalysisServiceHTTPServer(server, s.traffic)
	adminv1.RegisterTokenQuotaPolicyServiceHTTPServer(server, s.tokenQuota)
	adminv1.RegisterHealthServiceHTTPServer(server, s.health)
	adminv1.RegisterHeaderTransformationPolicyServiceHTTPServer(server, s.headerTransformation)
	adminv1.RegisterMockResponsePolicyServiceHTTPServer(server, s.mockResponse)
	adminv1.RegisterWasmPluginServiceHTTPServer(server, s.wasmPlugin)
	adminv1.RegisterPluginSourceServiceHTTPServer(server, s.pluginSource)
}
