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

// NewAdminAPIProxy 创建到 ingate-admin-api 的反向代理，保持控制台现有 API 路径与响应不变
func NewAdminAPIProxy(config *conf.Data_AdminAPI, logger *slog.Logger) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(strings.TrimSpace(config.GetBaseUrl()))
	if err != nil {
		return nil, fmt.Errorf("parse admin API base URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = func(response *http.Response) error {
		// Console 已将同一个请求 ID 写入响应，避免代理复制 Admin API Header 后出现重复值
		response.Header.Del(requestid.Header)
		return nil
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		logger.Error(
			"admin API proxy request failed",
			"request_id", request.Header.Get(requestid.Header),
			"method", request.Method,
			"path", request.URL.Path,
			"err", err,
		)
		if encodeErr := writeJSON(writer, http.StatusBadGateway, map[string]any{
			"code": http.StatusBadGateway,
			"msg":  "管理服务暂时不可用",
			"data": nil,
		}); encodeErr != nil {
			logger.Error(
				"write admin API proxy error response failed",
				"request_id", request.Header.Get(requestid.Header),
				"err", encodeErr,
			)
		}
	}
	return proxy, nil
}
