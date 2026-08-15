package server

import (
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
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

// HTTPHandlers 汇总需要注册到同一个 HTTP Server 的 Admin API 协议服务
type HTTPHandlers struct {
	authentication *authentication.Service
	gateway        *gateway.Service
	route          *route.Service
	upstream       *upstream.Service
	certificate    *certificate.Service
	rateLimit      *ratelimit.Service
	ipRestriction  *iprestriction.Service
	configuration  *configuration.Service
	health         *health.Service
}

// NewHTTPHandlers 创建 Admin API 的 HTTP 协议服务集合
func NewHTTPHandlers(
	authenticationService *authentication.Service,
	gatewayService *gateway.Service,
	routeService *route.Service,
	upstreamService *upstream.Service,
	certificateService *certificate.Service,
	rateLimitService *ratelimit.Service,
	ipRestrictionService *iprestriction.Service,
	configurationService *configuration.Service,
	healthService *health.Service,
) *HTTPHandlers {
	return &HTTPHandlers{
		authentication: authenticationService,
		gateway:        gatewayService,
		route:          routeService,
		upstream:       upstreamService,
		certificate:    certificateService,
		rateLimit:      rateLimitService,
		ipRestriction:  ipRestrictionService,
		configuration:  configurationService,
		health:         healthService,
	}
}

func (h *HTTPHandlers) register(server *kratoshttp.Server) {
	adminv1.RegisterAuthenticationServiceHTTPServer(server, h.authentication)
	adminv1.RegisterGatewayServiceHTTPServer(server, h.gateway)
	adminv1.RegisterRouteServiceHTTPServer(server, h.route)
	adminv1.RegisterUpstreamServiceHTTPServer(server, h.upstream)
	adminv1.RegisterCertificateServiceHTTPServer(server, h.certificate)
	adminv1.RegisterRateLimitPolicyServiceHTTPServer(server, h.rateLimit)
	adminv1.RegisterIPRestrictionPolicyServiceHTTPServer(server, h.ipRestriction)
	adminv1.RegisterConfigurationServiceHTTPServer(server, h.configuration)
	adminv1.RegisterHealthServiceHTTPServer(server, h.health)
}
