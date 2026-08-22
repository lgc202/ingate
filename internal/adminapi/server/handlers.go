package server

import (
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	aiusageservice "github.com/lgc202/ingate/internal/adminapi/service/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/service/caller"
	"github.com/lgc202/ingate/internal/adminapi/service/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	"github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	requestservice "github.com/lgc202/ingate/internal/adminapi/service/request"
	"github.com/lgc202/ingate/internal/adminapi/service/route"
	trafficservice "github.com/lgc202/ingate/internal/adminapi/service/traffic"
	"github.com/lgc202/ingate/internal/adminapi/service/upstream"
)

// HTTPHandlers 汇总需要注册到同一个 HTTP Server 的 Admin API 协议服务
type HTTPHandlers struct {
	aiUsage       *aiusageservice.Service
	caller        *caller.Service
	gateway       *gateway.Service
	route         *route.Service
	upstream      *upstream.Service
	certificate   *certificate.Service
	rateLimit     *ratelimit.Service
	ipRestriction *iprestriction.Service
	request       *requestservice.Service
	traffic       *trafficservice.Service
	health        *health.Service
}

// NewHTTPHandlers 创建 Admin API 的 HTTP 协议服务集合
func NewHTTPHandlers(
	aiUsageService *aiusageservice.Service,
	callerService *caller.Service,
	gatewayService *gateway.Service,
	routeService *route.Service,
	upstreamService *upstream.Service,
	certificateService *certificate.Service,
	rateLimitService *ratelimit.Service,
	ipRestrictionService *iprestriction.Service,
	requestService *requestservice.Service,
	trafficService *trafficservice.Service,
	healthService *health.Service,
) *HTTPHandlers {
	return &HTTPHandlers{
		aiUsage:       aiUsageService,
		caller:        callerService,
		gateway:       gatewayService,
		route:         routeService,
		upstream:      upstreamService,
		certificate:   certificateService,
		rateLimit:     rateLimitService,
		ipRestriction: ipRestrictionService,
		request:       requestService,
		traffic:       trafficService,
		health:        healthService,
	}
}

func (h *HTTPHandlers) register(server *kratoshttp.Server) {
	adminv1.RegisterAIUsageServiceHTTPServer(server, h.aiUsage)
	adminv1.RegisterCallerServiceHTTPServer(server, h.caller)
	adminv1.RegisterGatewayServiceHTTPServer(server, h.gateway)
	adminv1.RegisterRouteServiceHTTPServer(server, h.route)
	adminv1.RegisterUpstreamServiceHTTPServer(server, h.upstream)
	adminv1.RegisterCertificateServiceHTTPServer(server, h.certificate)
	adminv1.RegisterRateLimitPolicyServiceHTTPServer(server, h.rateLimit)
	adminv1.RegisterIPRestrictionPolicyServiceHTTPServer(server, h.ipRestriction)
	adminv1.RegisterRequestRecordServiceHTTPServer(server, h.request)
	adminv1.RegisterTrafficAnalysisServiceHTTPServer(server, h.traffic)
	adminv1.RegisterHealthServiceHTTPServer(server, h.health)
}
