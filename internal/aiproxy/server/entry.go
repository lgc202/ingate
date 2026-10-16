package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/lgc202/ingate/internal/aiproxy/handler"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// errorEncoder 写出入口适配阶段可安全公开的错误响应。
type errorEncoder func(http.ResponseWriter, *http.Request, error)

// trustedEntry 是模型与 MCP 共用的受保护 Unix 入口适配。信任来自套接字访问权限，
// 不是 Header；公共入口必须由 Envoy 覆盖这两个值，不能透传客户端的选择。
// invalidEntry 与 encode 由各协议入口提供，用于保持模型与 MCP 的公开错误形式。
func trustedEntry(invalidEntry error, encode errorEncoder) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ports := r.Header.Values(aidataplane.ListenerPortHeader)
			hosts := r.Header.Values(aidataplane.HostNameHeader)
			if len(ports) != 1 || len(hosts) != 1 || hosts[0] == "" || strings.ContainsAny(hosts[0], " ,\t\r\n") {
				encode(w, r, invalidEntry)
				return
			}

			port, err := strconv.ParseUint(ports[0], 10, 16)
			if err != nil || port == 0 {
				encode(w, r, invalidEntry)
				return
			}

			ctx := handler.WithEntry(r.Context(), int32(port), hosts[0])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
