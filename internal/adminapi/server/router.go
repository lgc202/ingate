package server

import (
	"github.com/gin-gonic/gin"

	resourcehandler "github.com/lgc202/ingate/internal/adminapi/handler/resource"
	"github.com/lgc202/ingate/internal/adminapi/pkg/middleware"
	resourceservice "github.com/lgc202/ingate/internal/adminapi/service/resource"
)

type apiPath string

const (
	apiPathAIRoutes          apiPath = "/ai-routes"
	apiPathAIProviders       apiPath = "/ai-providers"
	apiPathAIModels          apiPath = "/ai-models"
	apiPathAIPolicies        apiPath = "/ai-policies"
	apiPathPlugins           apiPath = "/plugins"
	apiPathPluginBindings    apiPath = "/plugin-bindings"
	apiPathAuthPolicies      apiPath = "/auth-policies"
	apiPathRateLimitPolicies apiPath = "/rate-limit-policies"
	apiPathPolicyBindings    apiPath = "/policy-bindings"
)

func (s *Server) router() *gin.Engine {
	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.CORS(),
		middleware.Recovery(s.logger),
	)

	handler := s.newHandler()
	router.GET("/healthz", handler.Health)

	api := router.Group("/api")
	api.GET("/gateways", handler.Gateway.List)
	api.GET("/gateways/:name/overview", handler.Gateway.Overview)
	api.GET("/gateways/:name", handler.Gateway.Get)
	api.GET("/routes", handler.Route.List)
	api.GET("/routes/:name", handler.Route.Get)
	api.GET("/upstreams", handler.Upstream.List)
	api.GET("/upstreams/:name", handler.Upstream.Get)
	api.GET("/runtime-snapshots", handler.Runtime.List)
	api.GET("/runtime-snapshots/:name", handler.Runtime.Get)
	s.registerResourceRoutes(api, apiPathAIRoutes, resourceservice.KindAIRoutes, handler.Resource)
	s.registerResourceRoutes(api, apiPathAIProviders, resourceservice.KindAIProviders, handler.Resource)
	s.registerResourceRoutes(api, apiPathAIModels, resourceservice.KindAIModels, handler.Resource)
	s.registerResourceRoutes(api, apiPathAIPolicies, resourceservice.KindAIPolicies, handler.Resource)
	s.registerResourceRoutes(api, apiPathPlugins, resourceservice.KindPlugins, handler.Resource)
	s.registerResourceRoutes(api, apiPathPluginBindings, resourceservice.KindPluginBindings, handler.Resource)
	s.registerResourceRoutes(api, apiPathAuthPolicies, resourceservice.KindAuthPolicies, handler.Resource)
	s.registerResourceRoutes(api, apiPathRateLimitPolicies, resourceservice.KindRateLimitPolicies, handler.Resource)
	s.registerResourceRoutes(api, apiPathPolicyBindings, resourceservice.KindPolicyBindings, handler.Resource)

	return router
}

func (s *Server) registerResourceRoutes(group *gin.RouterGroup, path apiPath, kind resourceservice.Kind, handler *resourcehandler.Handler) {
	group.GET(string(path), handler.List(kind))
	group.GET(string(path)+"/:name", handler.Get(kind))
}
