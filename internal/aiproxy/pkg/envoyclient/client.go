// Package envoyclient 构造仅连接指定 Envoy Unix 套接字的原生 HTTP Client。
// 模型选择、请求构造及响应写回分别由调用方承担，不提供另一套 Client 或流接口。
package envoyclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

// New 校验出口路径并创建连接池，不拨号，也不创建或删除套接字。
// 不读取环境代理、不解压响应、不跟随重定向，不设置截断 SSE 的整体请求期限。
// 调用方须在停止接收并排空请求后调用 Client.CloseIdleConnections。
func New(socket string) (*http.Client, error) {
	if !filepath.IsAbs(socket) || strings.ContainsRune(socket, 0) {
		return nil, errors.New("envoy egress socket must be an absolute path without NUL")
	}

	dialer := &net.Dialer{}
	transport := &http.Transport{
		// URL 只表达本地 HTTP 请求；无论其中地址为何，都只能连接这个 UDS。
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
		DisableCompression: true,
	}

	return &http.Client{
		Transport: transport,
		// 直接交付原始 3xx 和正文，不能再次请求 Location 指向的目标。
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}
