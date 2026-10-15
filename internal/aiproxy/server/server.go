// Package server 注册 AI Proxy 的原生 HTTP 路由及传输错误处理，不构造业务依赖。
package server

import (
	"net"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/handler"
)

// Handlers 保存当前实际实现的 AI Proxy HTTP 入口。
type Handlers struct {
	ModelProxy *handler.ModelProxyHandler
}

// New 注册 Chat Completions，但不开始接受请求，也不接管 Loader 的生命周期。
// 必须传入已有的 Unix Listener，不能退回 Kratos 的默认 TCP 监听。
// Listener 的创建、权限及失败清理由调用方负责；开始服务后由原生 Server.Stop 关闭。
func New(listener *net.UnixListener, handlers Handlers) *kratoshttp.Server {
	httpServer := newHTTP(listener)
	httpServer.Route("/").POST("/v1/chat/completions", handlers.ModelProxy.ChatCompletion)

	return httpServer
}
