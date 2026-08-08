package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// NewAdminAPIProxy 创建到 ingate-admin-api 的反向代理，保持控制台现有 API 路径与响应不变
func NewAdminAPIProxy(baseURL string, logger *slog.Logger) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
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
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusBadGateway)
		if encodeErr := json.NewEncoder(writer).Encode(map[string]any{
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
