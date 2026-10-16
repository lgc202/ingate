// Package server 注册 AI Proxy 的原生 HTTP 路由、Unix 入口适配及错误处理，不构造业务依赖。
package server

import (
	"net"
	"net/http"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/errno"
	"github.com/lgc202/ingate/internal/aiproxy/handler"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// Handlers 保存当前实际实现的 AI Proxy HTTP 入口。
type Handlers struct {
	ModelProxy *handler.ModelProxyHandler
	MCPProxy   *handler.MCPProxyHandler
}

// New 注册模型固定操作与 MCP 通配入口，但不开始接受请求或运行 Loader。
// 必须传入已有的 Unix Listener，不能退回 Kratos 的默认 TCP 监听。
// Listener 的创建、权限及失败清理由调用方负责；开始服务后由原生 Server.Stop 关闭。
func New(listener *net.UnixListener, handlers Handlers) *kratoshttp.Server {
	httpServer := newHTTP(listener)
	httpServer.Route("/").POST(aidataplane.ModelChatCompletionsPath, dispatchModel(handlers.ModelProxy.ChatCompletion, handlers.MCPProxy), modelEntry)
	httpServer.Route("/").POST(aidataplane.ModelResponsesPath, dispatchModel(handlers.ModelProxy.CreateResponse, handlers.MCPProxy), modelEntry)
	// MCP 入口路径由配置决定；固定模型路径也依赖 Envoy 已选择的协议，避免遮蔽 MCP-only 入口。
	// 不限定方法，让精确路径整体接管：非 POST 由 Handler 返回 405，不落普通 HTTP 兜底。
	httpServer.Route("/").Handle("*", "/{path:.*}", dispatchMCP(handlers.MCPProxy), trustedEntry(errno.ErrInvalidMCPEntry, encodeMCPError))

	return httpServer
}

// dispatchModel 仅在受保护 UDS 上采信 Envoy 覆盖的协议标记，不再读取一份配置。
func dispatchModel(model kratoshttp.HandlerFunc, mcp *handler.MCPProxyHandler) kratoshttp.HandlerFunc {
	return func(ctx kratoshttp.Context) error {
		if ctx.Request().Header.Get(aidataplane.EntryProtocolHeader) == "mcp" {
			return mcp.Forward(ctx)
		}

		return model(ctx)
	}
}

// modelEntry 在静态路径上根据 Envoy 的可信标记选择入口错误协议。
func modelEntry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(aidataplane.EntryProtocolHeader) == "mcp" {
			trustedEntry(errno.ErrInvalidMCPEntry, encodeMCPError)(next).ServeHTTP(w, r)
			return
		}

		trustedEntry(errno.ErrInvalidModelEntry, encodeModelError)(next).ServeHTTP(w, r)
	})
}

// dispatchMCP 只为 Envoy 明确选中的 MCP 入口接管其他方法，未知操作保持 404。
func dispatchMCP(mcp *handler.MCPProxyHandler) kratoshttp.HandlerFunc {
	return func(ctx kratoshttp.Context) error {
		if ctx.Request().Method != http.MethodPost && ctx.Request().Header.Get(aidataplane.EntryProtocolHeader) != "mcp" {
			encodeMCPError(ctx.Response(), ctx.Request(), errno.ErrMCPRouteNotFound)
			return nil
		}

		return mcp.Forward(ctx)
	}
}
