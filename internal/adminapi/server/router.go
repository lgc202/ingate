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
		middleware.Recovery(s.stdout),
	)

	handler := s.newHandler()
	router.GET("/healthz", handler.Health)

	api := router.Group("/api")
	api.GET("/gateways", handler.Gateway.List)
	api.GET("/gateways/:name", handler.Gateway.Get)
	api.GET("/routes", handler.Route.List)
	api.GET("/routes/:name", handler.Route.Get)
	api.GET("/upstreams", handler.Upstream.List)
	api.GET("/upstreams/:name", handler.Upstream.Get)
	api.GET("/runtime-snapshots", handler.Runtime.List)
	api.GET("/runtime-snapshots/:name", handler.Runtime.Get)

	return router
}
