package admin

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lgc202/ingate/internal/admin/handler"
	"github.com/lgc202/ingate/internal/admin/pkg/middleware"
	"github.com/lgc202/ingate/internal/pkg/httpserver"
)

func newHTTPServer(config Config, handlers *handler.Handler, logger *slog.Logger) *httpserver.Server {
	gin.SetMode(gin.ReleaseMode)
	return httpserver.New(config.Server.ListenAddress, newHTTPHandler(handlers, logger), logger)
}

// newHTTPHandler 注册管理 API 的中间件和路由
func newHTTPHandler(handlers *handler.Handler, logger *slog.Logger) http.Handler {
	router := gin.New()
	router.Use(
		middleware.RequestID(),
		middleware.Recovery(logger),
	)

	router.GET("/healthz", handlers.Health)

	apiV1 := router.Group("/api/v1")
	{
		apiV1.GET("/configuration/status", handlers.GetConfigurationStatus)

		accessKeys := apiV1.Group("/access-keys")
		{
			accessKeys.GET("", handlers.ListAccessKey)
			accessKeys.POST("", handlers.CreateAccessKey)
			accessKeys.PUT("/:id", handlers.UpdateAccessKey)
			accessKeys.PATCH("/:id/enabled", handlers.SetAccessKeyEnabled)
			accessKeys.POST("/:id/rotate", handlers.RotateAccessKey)
			accessKeys.DELETE("/:id", handlers.DeleteAccessKey)
		}

		certificates := apiV1.Group("/certificates")
		{
			certificates.GET("", handlers.ListCertificate)
			certificates.POST("", handlers.CreateCertificate)
			certificates.GET("/:id", handlers.GetCertificate)
			certificates.PUT("/:id", handlers.UpdateCertificate)
			certificates.DELETE("/:id", handlers.DeleteCertificate)
		}

		gateways := apiV1.Group("/gateways")
		{
			gateways.GET("", handlers.ListGateway)
			gateways.POST("", handlers.CreateGateway)
			gateways.GET("/:id", handlers.GetGateway)
			gateways.PUT("/:id", handlers.UpdateGateway)
			gateways.PATCH("/:id/enabled", handlers.SetGatewayEnabled)
			gateways.DELETE("/:id", handlers.DeleteGateway)
		}

		upstreams := apiV1.Group("/upstreams")
		{
			upstreams.GET("", handlers.ListUpstream)
			upstreams.POST("", handlers.CreateUpstream)
			upstreams.GET("/:id", handlers.GetUpstream)
			upstreams.PUT("/:id", handlers.UpdateUpstream)
			upstreams.DELETE("/:id", handlers.DeleteUpstream)
		}

		routes := apiV1.Group("/routes")
		{
			routes.GET("", handlers.ListRoute)
			routes.POST("", handlers.CreateRoute)
			routes.GET("/:id", handlers.GetRoute)
			routes.PUT("/:id", handlers.UpdateRoute)
			routes.PATCH("/:id/enabled", handlers.SetRouteEnabled)
			routes.DELETE("/:id", handlers.DeleteRoute)
		}

		rateLimitPolicies := apiV1.Group("/rate-limit-policies")
		{
			rateLimitPolicies.GET("", handlers.ListRateLimitPolicy)
			rateLimitPolicies.POST("", handlers.CreateRateLimitPolicy)
			rateLimitPolicies.GET("/:id", handlers.GetRateLimitPolicy)
			rateLimitPolicies.PUT("/:id", handlers.UpdateRateLimitPolicy)
			rateLimitPolicies.PATCH("/:id/enabled", handlers.SetRateLimitPolicyEnabled)
			rateLimitPolicies.DELETE("/:id", handlers.DeleteRateLimitPolicy)
		}

		accessControlPolicies := apiV1.Group("/access-control-policies")
		{
			accessControlPolicies.GET("", handlers.ListAccessControlPolicy)
			accessControlPolicies.POST("", handlers.CreateAccessControlPolicy)
			accessControlPolicies.GET("/:id", handlers.GetAccessControlPolicy)
			accessControlPolicies.PUT("/:id", handlers.UpdateAccessControlPolicy)
			accessControlPolicies.PATCH("/:id/enabled", handlers.SetAccessControlPolicyEnabled)
			accessControlPolicies.DELETE("/:id", handlers.DeleteAccessControlPolicy)
		}

		tokenQuotaPolicies := apiV1.Group("/token-quota-policies")
		{
			tokenQuotaPolicies.GET("", handlers.ListTokenQuotaPolicy)
			tokenQuotaPolicies.POST("", handlers.CreateTokenQuotaPolicy)
			tokenQuotaPolicies.GET("/:id", handlers.GetTokenQuotaPolicy)
			tokenQuotaPolicies.PUT("/:id", handlers.UpdateTokenQuotaPolicy)
			tokenQuotaPolicies.PATCH("/:id/enabled", handlers.SetTokenQuotaPolicyEnabled)
			tokenQuotaPolicies.DELETE("/:id", handlers.DeleteTokenQuotaPolicy)
		}
	}

	router.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(http.StatusNotFound, gin.H{"message": "api not found"})
	})

	return router
}
