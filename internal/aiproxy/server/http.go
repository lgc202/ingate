package server

import (
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/redis/go-redis/v9"

	"github.com/lgc202/ingate/internal/aiproxy/conf"
)

const tcpNetwork = "tcp"

// NewHTTPServer 创建 AI Proxy 的健康检查服务
func NewHTTPServer(config *conf.Server, rdb *redis.Client) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	httpServer := kratoshttp.NewServer(
		kratoshttp.Network(tcpNetwork),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
	)
	router := httpServer.Route("/")
	router.GET("/healthz", func(ctx kratoshttp.Context) error {
		return ctx.String(http.StatusOK, "ok\n")
	})
	router.GET("/readyz", func(ctx kratoshttp.Context) error {
		if err := rdb.Ping(ctx).Err(); err != nil {
			return ctx.String(http.StatusServiceUnavailable, "not ready\n")
		}
		return ctx.String(http.StatusOK, "ok\n")
	})
	return httpServer
}
