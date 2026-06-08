package server

import "github.com/gin-gonic/gin"

func (s *Server) router() *gin.Engine {
	router := gin.New()
	handler := s.newHandler()
	router.GET("/healthz", handler.Health)

	apiV1 := router.Group("/v1")
	{
		capabilities := apiV1.Group("/capabilities")
		{
			capabilities.POST("/rate-limit/check", handler.RateLimit.Check)
		}
	}
	return router
}
