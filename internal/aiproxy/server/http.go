package server

import (
	"net"
	"net/http"
	"net/url"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func newHTTP(listener *net.UnixListener) *kratoshttp.Server {
	return kratoshttp.NewServer(
		kratoshttp.Network("unix"),
		kratoshttp.Listener(listener),
		// 显式提供真实 UDS endpoint，避免框架把 Unix 地址当作 TCP 主机解析。
		kratoshttp.Endpoint(&url.URL{Scheme: "unix", Path: listener.Addr().String()}),
		// 关闭尾斜杠重定向：MCP 入口路径由配置决定，代理不得改写客户端请求路径。
		kratoshttp.StrictSlash(false),
		// 普通 Web 的默认请求期限不适用于 SSE；暂不设置整次请求期限。
		kratoshttp.Timeout(0),
		kratoshttp.ErrorEncoder(encodeModelError),
		// 业务入口只提供显式注册的模型路由，不回退到全局诊断路由。
		kratoshttp.NotFoundHandler(http.NotFoundHandler()),
		kratoshttp.MethodNotAllowedHandler(http.NotFoundHandler()),
	)
}
