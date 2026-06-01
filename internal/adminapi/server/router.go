package server

import (
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
		gateways := apiV1.Group("/gateways")
		{
			gateways.GET("", handler.Gateway.List)
			gateways.POST("", handler.Gateway.Create)
			gateways.GET("/:name/overview", handler.Gateway.Overview)
			gateways.GET("/:name", handler.Gateway.Get)
			gateways.PUT("/:name", handler.Gateway.Update)
			gateways.PATCH("/:name/enabled", handler.Gateway.SetEnabled)
			gateways.DELETE("/:name", handler.Gateway.Delete)
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

	return router
}
