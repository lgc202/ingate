package server

import (
	"net"
	"net/url"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func newHTTP(listener *net.UnixListener) *kratoshttp.Server {
	return kratoshttp.NewServer(
		kratoshttp.Network("unix"),
		kratoshttp.Listener(listener),
		// 显式提供真实 UDS endpoint，避免框架把 Unix 地址当作 TCP 主机解析。
		kratoshttp.Endpoint(&url.URL{Scheme: "unix", Path: listener.Addr().String()}),
		// 普通 Web 的默认请求期限不适用于 SSE；暂不设置整次请求期限。
		kratoshttp.Timeout(0),
		// 当前保留已映射的 Kratos 错误编码，不把它确定为最终模型错误协议。
		kratoshttp.ErrorEncoder(kratoshttp.DefaultErrorEncoder),
	)
}
