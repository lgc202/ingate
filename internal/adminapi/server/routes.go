package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler"
)

const (
	adminV1Path            = "/admin/v1"
	gatewaysPath           = "/gateways"
	backendsPath           = "/backends"
	routesPath             = "/routes"
	certificatesPath       = "/certificates"
	certificateSecretsPath = "/certificate-secrets"
	authPoliciesPath       = "/auth-policies"
	trafficPoliciesPath    = "/traffic-policies"
	overviewPath           = "/overview"
	topologyPath           = "/topology"
	eventsPath             = "/events"
	resourceNamePath       = "/:name"
)

type handlers struct {
	health        *handler.HealthHandler
	gateway       *handler.GatewayHandler
	backend       *handler.BackendHandler
	route         *handler.RouteHandler
	certificate   *handler.CertificateHandler
	authPolicy    *handler.AuthPolicyHandler
	trafficPolicy *handler.TrafficPolicyHandler
	overview      *handler.OverviewHandler
	topology      *handler.TopologyHandler
	event         *handler.EventHandler
}

func newRouter(handlers handlers, adminToken string) http.Handler {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), requestIDMiddleware(), localDevelopmentCORSMiddleware())

	router.GET("/healthz", handlers.health.Healthz)
	router.GET("/readyz", handlers.health.Readyz)

	adminV1 := router.Group(adminV1Path)
	adminV1.Use(bearerTokenAuthMiddleware(adminToken))
	{
		adminV1.GET(overviewPath, handlers.overview.GetOverview)
		adminV1.GET(topologyPath, handlers.topology.GetTopology)
		adminV1.GET(eventsPath, handlers.event.List)

		adminV1.POST(gatewaysPath, handlers.gateway.Create)
		adminV1.GET(gatewaysPath, handlers.gateway.List)
		adminV1.GET(gatewaysPath+resourceNamePath, handlers.gateway.Get)
		adminV1.PUT(gatewaysPath+resourceNamePath, handlers.gateway.Update)
		adminV1.DELETE(gatewaysPath+resourceNamePath, handlers.gateway.Delete)

		adminV1.POST(backendsPath, handlers.backend.Create)
		adminV1.GET(backendsPath, handlers.backend.List)
		adminV1.GET(backendsPath+resourceNamePath, handlers.backend.Get)
		adminV1.PUT(backendsPath+resourceNamePath, handlers.backend.Update)
		adminV1.DELETE(backendsPath+resourceNamePath, handlers.backend.Delete)

		adminV1.POST(routesPath, handlers.route.Create)
		adminV1.GET(routesPath, handlers.route.List)
		adminV1.GET(routesPath+resourceNamePath, handlers.route.Get)
		adminV1.PUT(routesPath+resourceNamePath, handlers.route.Update)
		adminV1.DELETE(routesPath+resourceNamePath, handlers.route.Delete)

		adminV1.POST(certificatesPath, handlers.certificate.Create)
		adminV1.GET(certificatesPath, handlers.certificate.List)
		adminV1.GET(certificateSecretsPath, handlers.certificate.ListSecrets)
		adminV1.GET(certificatesPath+resourceNamePath, handlers.certificate.Get)
		adminV1.PUT(certificatesPath+resourceNamePath, handlers.certificate.Update)
		adminV1.DELETE(certificatesPath+resourceNamePath, handlers.certificate.Delete)

		adminV1.POST(authPoliciesPath, handlers.authPolicy.Create)
		adminV1.GET(authPoliciesPath, handlers.authPolicy.List)
		adminV1.GET(authPoliciesPath+resourceNamePath, handlers.authPolicy.Get)
		adminV1.PUT(authPoliciesPath+resourceNamePath, handlers.authPolicy.Update)
		adminV1.DELETE(authPoliciesPath+resourceNamePath, handlers.authPolicy.Delete)

		adminV1.POST(trafficPoliciesPath, handlers.trafficPolicy.Create)
		adminV1.GET(trafficPoliciesPath, handlers.trafficPolicy.List)
		adminV1.GET(trafficPoliciesPath+resourceNamePath, handlers.trafficPolicy.Get)
		adminV1.PUT(trafficPoliciesPath+resourceNamePath, handlers.trafficPolicy.Update)
		adminV1.DELETE(trafficPoliciesPath+resourceNamePath, handlers.trafficPolicy.Delete)
	}

	return router
}
