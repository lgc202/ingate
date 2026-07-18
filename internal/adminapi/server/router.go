package server

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/pkg/middleware"
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

	apiV1 := router.Group("/api/v1")
	{
		certificates := apiV1.Group("/certificates")
		{
			certificates.GET("", handler.Certificate.List)
			certificates.POST("", handler.Certificate.Create)
			certificates.GET("/:id", handler.Certificate.Get)
			certificates.PUT("/:id", handler.Certificate.Update)
			certificates.DELETE("/:id", handler.Certificate.Delete)
		}

		gateways := apiV1.Group("/gateways")
		{
			gateways.GET("", handler.Gateway.List)
			gateways.POST("", handler.Gateway.Create)
			gateways.GET("/:id", handler.Gateway.Get)
			gateways.PUT("/:id", handler.Gateway.Update)
			gateways.PATCH("/:id/enabled", handler.Gateway.SetEnabled)
			gateways.DELETE("/:id", handler.Gateway.Delete)
		}

		upstreams := apiV1.Group("/upstreams")
		{
			upstreams.GET("", handler.Upstream.List)
			upstreams.POST("", handler.Upstream.Create)
			upstreams.GET("/:id", handler.Upstream.Get)
			upstreams.PUT("/:id", handler.Upstream.Update)
			upstreams.DELETE("/:id", handler.Upstream.Delete)
		}

		routes := apiV1.Group("/routes")
		{
			routes.GET("", handler.Route.List)
			routes.POST("", handler.Route.Create)
			routes.GET("/:id", handler.Route.Get)
			routes.PUT("/:id", handler.Route.Update)
			routes.PATCH("/:id/enabled", handler.Route.SetEnabled)
			routes.DELETE("/:id", handler.Route.Delete)
		}

		rateLimitPolicies := apiV1.Group("/rate-limit-policies")
		{
			rateLimitPolicies.GET("", handler.RateLimitPolicy.List)
			rateLimitPolicies.POST("", handler.RateLimitPolicy.Create)
			rateLimitPolicies.GET("/:id", handler.RateLimitPolicy.Get)
			rateLimitPolicies.PUT("/:id", handler.RateLimitPolicy.Update)
			rateLimitPolicies.PATCH("/:id/enabled", handler.RateLimitPolicy.SetEnabled)
			rateLimitPolicies.DELETE("/:id", handler.RateLimitPolicy.Delete)
		}

		accessControlPolicies := apiV1.Group("/access-control-policies")
		{
			accessControlPolicies.GET("", handler.AccessControlPolicy.List)
			accessControlPolicies.POST("", handler.AccessControlPolicy.Create)
			accessControlPolicies.GET("/:id", handler.AccessControlPolicy.Get)
			accessControlPolicies.PUT("/:id", handler.AccessControlPolicy.Update)
			accessControlPolicies.PATCH("/:id/enabled", handler.AccessControlPolicy.SetEnabled)
			accessControlPolicies.DELETE("/:id", handler.AccessControlPolicy.Delete)
		}
	}

	s.mountConsole(router)

	return router
}

func (s *Server) mountConsole(router *gin.Engine) {
	if s.consoleDir == "" {
		return
	}

	router.StaticFS("/assets", http.Dir(filepath.Join(s.consoleDir, "assets")))
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"message": "api not found"})
			return
		}
		c.File(filepath.Join(s.consoleDir, "index.html"))
	})
}
