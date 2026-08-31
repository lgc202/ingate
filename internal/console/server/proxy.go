package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/lgc202/ingate/internal/console/conf"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// AdminAPIProxy 将控制台管理请求转发到 ingate-admin-api。
type AdminAPIProxy struct {
	*httputil.ReverseProxy
}

// AssistantProxy 将运维助手请求转发到 ingate-assistant。
type AssistantProxy struct {
	*httputil.ReverseProxy
}

// NewAdminAPIProxy 创建到 ingate-admin-api 的反向代理，保持控制台现有 API 路径与响应不变。
func NewAdminAPIProxy(config *conf.Data_AdminAPI, logger *slog.Logger) (*AdminAPIProxy, error) {
	proxy, err := newReverseProxy(config.GetBaseUrl(), "admin API", "管理服务暂时不可用", logger)
	if err != nil {
		return nil, err
	}
	return &AdminAPIProxy{ReverseProxy: proxy}, nil
}

// NewAssistantProxy 创建到 ingate-assistant 的反向代理；SSE 响应由标准 ReverseProxy 逐段刷新。
func NewAssistantProxy(config *conf.Data_Assistant, logger *slog.Logger) (*AssistantProxy, error) {
	proxy, err := newReverseProxy(config.GetBaseUrl(), "assistant", "运维助手暂时不可用", logger)
	if err != nil {
		return nil, err
	}
	proxy.FlushInterval = -1
	return &AssistantProxy{ReverseProxy: proxy}, nil
}

func newReverseProxy(
	baseURL string,
	service string,
	userMessage string,
	logger *slog.Logger,
) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse %s base URL: %w", service, err)
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			// Rewrite 会先移除客户端伪造的 Forwarded Header，并让 Host 与后端地址一致。
			request.SetURL(target)
			request.SetXForwarded()
			// 后端使用可信管理员 Header，不需要接触浏览器会话或其他 Authorization 凭据。
			request.Out.Header.Del("Cookie")
			request.Out.Header.Del("Authorization")
		},
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		response.Header.Set("Cache-Control", "no-store")
		// Console 已将同一个请求 ID 写入响应，避免复制后端 Header 后出现重复值。
		response.Header.Del(requestid.Header)
		// 内部服务不得覆盖 Console 的会话 Cookie，也不向浏览器暴露后端实现标识。
		response.Header.Del("Set-Cookie")
		response.Header.Del("Server")
		return nil
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		logger.ErrorContext(
			request.Context(),
			"backend proxy request failed",
			"backend", service,
			"request_id", request.Header.Get(requestid.Header),
			"method", request.Method,
			"path", request.URL.Path,
			"err", err,
		)
		writeResponse(writer, http.StatusBadGateway, userMessage, nil)
	}
	return proxy, nil
}
