// Package aiproxy 装配 AI Proxy 的模型请求链，不拥有领域资源或配置编译。
package aiproxy

import (
	"fmt"
	"net"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/handler"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/aiconfig"
	"github.com/lgc202/ingate/internal/aiproxy/pkg/envoyclient"
	"github.com/lgc202/ingate/internal/aiproxy/server"
	"github.com/lgc202/ingate/internal/aiproxy/service/modelproxy"
)

// NewHTTPServer 将原生出站 Client、完整模型 Service 和 Handler 装配到 Kratos HTTP Server。
// 调用方拥有 Listener 与 Loader：负责保护入口 socket 及目录、运行 Loader，
// 并在停止接收、排空 Server 后调用返回的清理函数释放出口空闲连接。
// 构造不开始接受请求或启动 Watch；失败不关闭调用方的 Listener 或 Loader。
// Listener 与 Loader 由调用方的成功构造链保证有效。
// Server 从 Envoy 覆盖的内部 Header 恢复入口；入口信任依赖调用方保护 Unix socket。
func NewHTTPServer(
	listener *net.UnixListener,
	loader *aiconfig.Loader,
	egressSocket string,
	maxRequestBodyBytes int64,
) (*kratoshttp.Server, func(), error) {
	client, err := envoyclient.New(egressSocket)
	if err != nil {
		return nil, nil, fmt.Errorf("create Envoy HTTP client: %w", err)
	}
	service := modelproxy.New(loader, client)
	modelProxyHandler, err := handler.NewModelProxy(service, maxRequestBodyBytes)
	if err != nil {
		client.CloseIdleConnections()
		return nil, nil, fmt.Errorf("create model handler: %w", err)
	}

	httpServer := server.New(listener, server.Handlers{ModelProxy: modelProxyHandler})

	return httpServer, client.CloseIdleConnections, nil
}
