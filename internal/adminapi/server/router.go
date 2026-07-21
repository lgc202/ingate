package server

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/adminapi/handler"
	"github.com/lgc202/ingate/internal/adminapi/pkg/middleware"
)

// NewRouter 创建管理 API 与控制台静态资源路由
func NewRouter(handlers *handler.Handler, consoleDir string, logger *slog.Logger) http.Handler {
	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.CORS(),
		middleware.Recovery(logger),
	)

	router.GET("/healthz", handlers.Health)

	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/configuration/status", handlers.ConfigurationStatus.Get)

		certificates := apiV1.Group("/certificates")
		{
			certificates.GET("", handlers.Certificate.List)
			certificates.POST("", handlers.Certificate.Create)
			certificates.GET("/:id", handlers.Certificate.Get)
			certificates.PUT("/:id", handlers.Certificate.Update)
			certificates.DELETE("/:id", handlers.Certificate.Delete)
		}

		gateways := apiV1.Group("/gateways")
		{
			gateways.GET("", handlers.Gateway.List)
			gateways.POST("", handlers.Gateway.Create)
			gateways.GET("/:id", handlers.Gateway.Get)
			gateways.PUT("/:id", handlers.Gateway.Update)
			gateways.PATCH("/:id/enabled", handlers.Gateway.SetEnabled)
			gateways.DELETE("/:id", handlers.Gateway.Delete)
		}

		upstreams := apiV1.Group("/upstreams")
		{
			upstreams.GET("", handlers.Upstream.List)
			upstreams.POST("", handlers.Upstream.Create)
			upstreams.GET("/:id", handlers.Upstream.Get)
			upstreams.PUT("/:id", handlers.Upstream.Update)
			upstreams.DELETE("/:id", handlers.Upstream.Delete)
		}

		routes := apiV1.Group("/routes")
		{
			routes.GET("", handlers.Route.List)
			routes.POST("", handlers.Route.Create)
			routes.GET("/:id", handlers.Route.Get)
			routes.PUT("/:id", handlers.Route.Update)
			routes.PATCH("/:id/enabled", handlers.Route.SetEnabled)
			routes.DELETE("/:id", handlers.Route.Delete)
		}

		rateLimitPolicies := apiV1.Group("/rate-limit-policies")
		{
			rateLimitPolicies.GET("", handlers.RateLimitPolicy.List)
			rateLimitPolicies.POST("", handlers.RateLimitPolicy.Create)
			rateLimitPolicies.GET("/:id", handlers.RateLimitPolicy.Get)
			rateLimitPolicies.PUT("/:id", handlers.RateLimitPolicy.Update)
			rateLimitPolicies.PATCH("/:id/enabled", handlers.RateLimitPolicy.SetEnabled)
			rateLimitPolicies.DELETE("/:id", handlers.RateLimitPolicy.Delete)
		}

		accessControlPolicies := apiV1.Group("/access-control-policies")
		{
			accessControlPolicies.GET("", handlers.AccessControlPolicy.List)
			accessControlPolicies.POST("", handlers.AccessControlPolicy.Create)
			accessControlPolicies.GET("/:id", handlers.AccessControlPolicy.Get)
			accessControlPolicies.PUT("/:id", handlers.AccessControlPolicy.Update)
			accessControlPolicies.PATCH("/:id/enabled", handlers.AccessControlPolicy.SetEnabled)
			accessControlPolicies.DELETE("/:id", handlers.AccessControlPolicy.Delete)
		}
	}

	mountConsole(router, consoleDir)

	return router
}

func mountConsole(router *gin.Engine, consoleDir string) {
	if consoleDir == "" {
		return
	}

	router.StaticFS("/assets", http.Dir(filepath.Join(consoleDir, "assets")))
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"message": "api not found"})
			return
		}
		c.File(filepath.Join(consoleDir, "index.html"))
	})
}
