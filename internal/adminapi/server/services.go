package server

import (
	aiusageservice "github.com/lgc202/ingate/internal/adminapi/service/aiusage"
	"github.com/lgc202/ingate/internal/adminapi/service/caller"
	"github.com/lgc202/ingate/internal/adminapi/service/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	"github.com/lgc202/ingate/internal/adminapi/service/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/service/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/service/pluginsource"
	"github.com/lgc202/ingate/internal/adminapi/service/ratelimit"
	requestservice "github.com/lgc202/ingate/internal/adminapi/service/request"
	"github.com/lgc202/ingate/internal/adminapi/service/route"
	"github.com/lgc202/ingate/internal/adminapi/service/tokenquota"
	trafficservice "github.com/lgc202/ingate/internal/adminapi/service/traffic"
	"github.com/lgc202/ingate/internal/adminapi/service/upstream"
	"github.com/lgc202/ingate/internal/adminapi/service/wasmplugin"
)

// Services 汇总由 HTTP 和 gRPC transport 共同发布的 Admin API 协议服务。
// 两种 transport 只负责协议接入，所有业务规则仍由同一组 service/biz 实例执行。
type Services struct {
	aiUsage              *aiusageservice.Service
	caller               *caller.Service
	gateway              *gateway.Service
	route                *route.Service
	upstream             *upstream.Service
	certificate          *certificate.Service
	rateLimit            *ratelimit.Service
	ipRestriction        *iprestriction.Service
	request              *requestservice.Service
	traffic              *trafficservice.Service
	tokenQuota           *tokenquota.Service
	health               *health.Service
	headerTransformation *headertransformation.Service
	mockResponse         *mockresponse.Service
	wasmPlugin           *wasmplugin.Service
	pluginSource         *pluginsource.Service
}

// NewServices 创建 Admin API 的协议服务集合。
func NewServices(
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
	tokenQuotaService *tokenquota.Service,
	healthService *health.Service,
	headerTransformationService *headertransformation.Service,
	mockResponseService *mockresponse.Service,
	wasmPluginService *wasmplugin.Service,
	pluginSourceService *pluginsource.Service,
) *Services {
	return &Services{
		aiUsage:              aiUsageService,
		caller:               callerService,
		gateway:              gatewayService,
		route:                routeService,
		upstream:             upstreamService,
		certificate:          certificateService,
		rateLimit:            rateLimitService,
		ipRestriction:        ipRestrictionService,
		request:              requestService,
		traffic:              trafficService,
		tokenQuota:           tokenQuotaService,
		health:               healthService,
		headerTransformation: headerTransformationService,
		mockResponse:         mockResponseService,
		wasmPlugin:           wasmPluginService,
		pluginSource:         pluginSourceService,
	}
}
