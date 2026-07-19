package policy

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/pkg/llm"
	config "github.com/lgc202/ingate/pkg/plugin/aiproxy"
)

const chatCompletionsPath = "/v1/chat/completions"

// ValidateEndpoint 校验第一版 AI Proxy 支持的请求方法和路径
func (r *Runner) ValidateEndpoint(req Request) *Rejection {
	if req.Method != "POST" {
		return &Rejection{
			StatusCode: 405,
			Message:    "Only POST is supported for this endpoint",
			Type:       "invalid_request_error",
			Code:       "method_not_allowed",
			Allow:      "POST",
		}
	}
	path, _, _ := strings.Cut(req.Path, "?")
	if path != chatCompletionsPath {
		return &Rejection{
			StatusCode: 404,
			Message:    "The requested endpoint is not supported",
			Type:       "invalid_request_error",
			Code:       "endpoint_not_found",
		}
	}
	return nil
}

// Apply 校验客户端请求并选择公开模型绑定的目标
func (r *Runner) Apply(models map[string]config.ModelConfig, req Request) Decision {
	chat, err := llm.DecodeChatRequest(req.Body)
	if err == nil {
		err = chat.ValidateSupported()
	}
	if err != nil {
		code := "invalid_request"
		if errors.Is(err, llm.ErrUnsupportedFeature) {
			code = "unsupported_feature"
		}
		return Decision{Rejection: &Rejection{
			StatusCode: 400,
			Message:    err.Error(),
			Type:       "invalid_request_error",
			Code:       code,
		}}
	}

	model, exists := models[chat.Model]
	if !exists {
		return Decision{Rejection: &Rejection{
			StatusCode: 404,
			Message:    fmt.Sprintf("The model %q does not exist or is not available for this route", chat.Model),
			Type:       "invalid_request_error",
			Param:      "model",
			Code:       "model_not_found",
		}}
	}
	return Decision{Selection: &Selection{Model: model, Stream: chat.Streaming()}}
}

// RequestTooLarge 返回请求体超过插件缓冲上限时的错误
func (r *Runner) RequestTooLarge() *Rejection {
	return &Rejection{
		StatusCode: 413,
		Message:    "Request body is too large",
		Type:       "invalid_request_error",
		Code:       "request_too_large",
	}
}

// InternalError 返回不向客户端暴露内部细节的错误
func (r *Runner) InternalError() *Rejection {
	return &Rejection{
		StatusCode: 500,
		Message:    "The request could not be processed",
		Type:       "server_error",
		Code:       "internal_error",
	}
}
