// Package modelendpoint 检查运维助手配置的模型服务是否可用。
package modelendpoint

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/lgc202/ingate/internal/assistant/conf"
)

// Endpoint 使用不携带凭据的健康地址检查模型服务。
type Endpoint struct {
	client    *http.Client
	healthURL string
}

// New 创建模型服务健康检查器。
func New(config *conf.Model) *Endpoint {
	return &Endpoint{
		client:    &http.Client{Timeout: config.GetHealthTimeout().AsDuration()},
		healthURL: config.GetHealthUrl(),
	}
}

// Check 确认模型健康地址返回成功的 HTTP 状态码。
func (e *Endpoint) Check(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, e.healthURL, nil)
	if err != nil {
		return fmt.Errorf("create model health request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := e.client.Do(request)
	if err != nil {
		return fmt.Errorf("request model health endpoint: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("model health endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}
