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
		apiV1.GET("/route-policy-capabilities", handler.Route.PolicyCapabilities)
		apiV1.GET("/runtime-groups", handler.RuntimeGroup.List)

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
			upstreams.GET("/:name", handler.Upstream.Get)
			upstreams.PUT("/:name", handler.Upstream.Update)
			upstreams.DELETE("/:name", handler.Upstream.Delete)
		}

		routes := apiV1.Group("/routes")
		{
			routes.GET("", handler.Route.List)
			routes.POST("", handler.Route.Create)
			routes.GET("/:name", handler.Route.Get)
			routes.PUT("/:name", handler.Route.Update)
			routes.PATCH("/:name/enabled", handler.Route.SetEnabled)
			routes.DELETE("/:name", handler.Route.Delete)
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
