package server

import (
	"net/http"
	"strconv"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/lgc202/ingate/internal/aiproxy/handler"
	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// modelEntry 仅挂在受保护 Unix Server 的模型路由上。信任来自套接字访问权限，
// 不是 Header；公共入口必须由 Envoy 覆盖这两个值，不能透传客户端的选择。
func modelEntry(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ports := r.Header.Values(aidataplane.ListenerPortHeader)
		hosts := r.Header.Values(aidataplane.HostNameHeader)
		if len(ports) != 1 || len(hosts) != 1 || hosts[0] == "" || strings.ContainsAny(hosts[0], " ,\t\r\n") {
			kratoshttp.DefaultErrorEncoder(w, r, kratoserrors.BadRequest("INVALID_MODEL_ENTRY", "invalid model entry"))
			return
		}
		port, err := strconv.ParseUint(ports[0], 10, 16)
		if err != nil || port == 0 {
			kratoshttp.DefaultErrorEncoder(w, r, kratoserrors.BadRequest("INVALID_MODEL_ENTRY", "invalid model entry"))
			return
		}

		ctx := handler.WithModelEntry(r.Context(), int32(port), hosts[0])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
