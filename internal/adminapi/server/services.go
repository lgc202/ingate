package server

import (
	aiusageservice "github.com/lgc202/ingate/internal/adminapi/service/analytics/aiusage"
	requestservice "github.com/lgc202/ingate/internal/adminapi/service/analytics/requestrecord"
	trafficservice "github.com/lgc202/ingate/internal/adminapi/service/analytics/traffic"
	"github.com/lgc202/ingate/internal/adminapi/service/caller"
	"github.com/lgc202/ingate/internal/adminapi/service/health"
	sourceservice "github.com/lgc202/ingate/internal/adminapi/service/plugin/source"
	wasmservice "github.com/lgc202/ingate/internal/adminapi/service/plugin/wasm"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/headertransformation"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/iprestriction"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/mockresponse"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/ratelimit"
	"github.com/lgc202/ingate/internal/adminapi/service/policy/tokenquota"
	"github.com/lgc202/ingate/internal/adminapi/service/routing/certificate"
	"github.com/lgc202/ingate/internal/adminapi/service/routing/gateway"
	"github.com/lgc202/ingate/internal/adminapi/service/routing/route"
	routingservice "github.com/lgc202/ingate/internal/adminapi/service/routing/service"
)

// Services 汇总由 HTTP 和 gRPC transport 共同发布的 Admin API 协议服务。
// 两种 transport 只负责协议接入，所有业务规则仍由同一组 service/biz 实例执行。
type Services struct {
	health *health.Service
	caller *caller.Service

	gateway           *gateway.Service
	route             *route.Service
	serviceManagement *routingservice.Service
	certificate       *certificate.Service

	rateLimit            *ratelimit.Service
	ipRestriction        *iprestriction.Service
	tokenQuota           *tokenquota.Service
	headerTransformation *headertransformation.Service
	mockResponse         *mockresponse.Service

	aiUsage         *aiusageservice.Service
	requestRecord   *requestservice.Service
	trafficAnalysis *trafficservice.Service

	wasmPlugin   *wasmservice.Service
	pluginSource *sourceservice.Service
}

// NewServices 创建 Admin API 的协议服务集合。
func NewServices(
	aiUsageService *aiusageservice.Service,
	callerService *caller.Service,
	gatewayService *gateway.Service,
	routeService *route.Service,
	serviceManagementService *routingservice.Service,
	certificateService *certificate.Service,
	rateLimitService *ratelimit.Service,
	ipRestrictionService *iprestriction.Service,
	requestRecordService *requestservice.Service,
	trafficAnalysisService *trafficservice.Service,
	tokenQuotaService *tokenquota.Service,
	healthService *health.Service,
	headerTransformationService *headertransformation.Service,
	mockResponseService *mockresponse.Service,
	wasmPluginService *wasmservice.Service,
	pluginSourceService *sourceservice.Service,
) *Services {
	return &Services{
		health: healthService,
		caller: callerService,

		gateway:           gatewayService,
		route:             routeService,
		serviceManagement: serviceManagementService,
		certificate:       certificateService,

		rateLimit:            rateLimitService,
		ipRestriction:        ipRestrictionService,
		tokenQuota:           tokenQuotaService,
		headerTransformation: headerTransformationService,
		mockResponse:         mockResponseService,

		aiUsage:         aiUsageService,
		requestRecord:   requestRecordService,
		trafficAnalysis: trafficAnalysisService,

		wasmPlugin:   wasmPluginService,
		pluginSource: pluginSourceService,
	}
}
