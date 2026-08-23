package server

import (
	"log/slog"
	"net/http/httputil"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/console/conf"
)

// NewHTTPServer 创建控制台静态资源和管理 API 代理服务
func NewHTTPServer(
	config *conf.Server,
	adminAPIProxy *httputil.ReverseProxy,
	auth *SessionAuth,
	logger *slog.Logger,
) *kratoshttp.Server {
	httpConfig := config.GetHttp()
	server := kratoshttp.NewServer(
		kratoshttp.Network("tcp"),
		kratoshttp.Address(httpConfig.GetAddr()),
		kratoshttp.Timeout(httpConfig.GetTimeout().AsDuration()),
		kratoshttp.Filter(
			recovery(logger),
			requestID(),
		),
	)
	server.HandlePrefix("/", NewRouter(adminAPIProxy, auth, config.GetConsoleDir(), logger))
	return server
}
