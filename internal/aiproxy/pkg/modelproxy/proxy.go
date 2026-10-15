// Package modelproxy 使用标准 ReverseProxy 将已准备的模型请求交给受限 Envoy 出口。
// 本包不选择模型、不读取配置、不连接提供商，也不决定客户端错误 envelope。
package modelproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/aidataplane"
)

// ErrTargetUnavailable 表示请求没有可信目标或无法构造选定目标的操作路径。
var ErrTargetUnavailable = errors.New("model proxy target unavailable")

type targetKey struct{}

// outboundTarget 只在一次 ServeHTTP 调用内传给 Rewrite，不包含共享可变状态。
type outboundTarget struct {
	name string
	url  url.URL
}

// Proxy 持有仅能连接一个 UDS 的 Transport，可并发转发普通响应和 SSE。
// socket 及其目录权限由部署保证；固定地址不是对本地调用者的认证。
type Proxy struct {
	transport *http.Transport
	reverse   *httputil.ReverseProxy
	onError   func(http.ResponseWriter, *http.Request, error)
}

// New 创建出站转发器但不监听、不发起连接。onError 必须编码安全的客户端错误，
// 不得直接返回底层错误文本；流已开始后的中断由标准 ReverseProxy 终止，不能再写 JSON。
// 组件停止接收并排空请求后，应调用 CloseIdleConnections 释放连接池。
func New(
	socket string,
	onError func(http.ResponseWriter, *http.Request, error),
) (*Proxy, error) {
	if !filepath.IsAbs(socket) || strings.ContainsRune(socket, 0) {
		return nil, errors.New("model egress socket must be an absolute path without NUL")
	}
	if onError == nil {
		return nil, errors.New("model proxy error handler is required")
	}

	dialer := &net.Dialer{}
	transport := &http.Transport{
		// 不读取环境代理，不解析请求 Host，也没有 TCP 或直连提供商的回退。
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
		DisableCompression: true,
	}
	p := &Proxy{transport: transport, onError: onError}
	p.reverse = &httputil.ReverseProxy{
		Transport:    transport,
		Rewrite:      p.rewrite,
		ErrorHandler: onError,
	}

	return p, nil
}

// ServeHTTP 只转发已准备的 Chat Completions POST，不采用客户端的 URL、Host 或查询参数。
// 上游路径由所选 schema.prefix 加操作后缀构造；仅允许 Accept 与 Content-Type 进入出口，
// 因而客户端凭据、Cookie、Forwarded、Envoy 控制 Header 和内部标识均不透传。
func (p *Proxy) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	backend, ok := Backend(request.Context())
	if !ok || backend.TargetName == "" || request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
		p.onError(response, request, ErrTargetUnavailable)
		return
	}

	path, err := url.JoinPath("/", backend.Schema.Prefix, "chat/completions")
	if err != nil {
		p.onError(response, request, fmt.Errorf("%w: %w", ErrTargetUnavailable, err))
		return
	}
	target, err := url.ParseRequestURI(path)
	if err != nil || target.IsAbs() || target.Host != "" || target.RawQuery != "" || target.Fragment != "" {
		p.onError(response, request, ErrTargetUnavailable)
		return
	}

	target.Scheme = "http"
	target.Host = "ingate-ai-egress"
	ctx := context.WithValue(request.Context(), targetKey{}, outboundTarget{name: backend.TargetName, url: *target})

	p.reverse.ServeHTTP(response, request.Clone(ctx))
}

// CloseIdleConnections 释放空闲 UDS 连接，不取消在途请求或代替 Server 排空。
func (p *Proxy) CloseIdleConnections() {
	p.transport.CloseIdleConnections()
}

func (p *Proxy) rewrite(request *httputil.ProxyRequest) {
	target := request.In.Context().Value(targetKey{}).(outboundTarget)
	request.Out.URL = &target.url
	request.Out.Host = target.url.Host
	request.Out.Header = make(http.Header)
	request.Out.Header.Set("Content-Type", "application/json")
	request.Out.Header.Set(aidataplane.TargetHeader, target.name)
	request.Out.Header["Accept"] = append([]string(nil), request.In.Header.Values("Accept")...)
	request.Out.Header["User-Agent"] = []string{""} // 不透传客户端身份，也不让默认客户端补 Go User-Agent。
	request.Out.GetBody = nil
	request.Out.Trailer = nil
	request.Out.TransferEncoding = nil
}
