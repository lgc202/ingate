package policy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

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

// Apply 选择客户端请求的模型并将 model 改写为上游模型名称
func (r *Runner) Apply(models map[string]config.ModelConfig, req Request) Decision {
	fields, rejection := requestFields(req.Body)
	if rejection != nil {
		return Decision{Rejection: rejection}
	}

	rawModel, exists := fields["model"]
	if !exists {
		return Decision{Rejection: invalidModel("Field 'model' is required", "model_required")}
	}
	var modelName string
	if err := json.Unmarshal(rawModel, &modelName); err != nil || strings.TrimSpace(modelName) == "" {
		return Decision{Rejection: invalidModel("Field 'model' must be a non-empty string", "invalid_model")}
	}

	model, exists := models[modelName]
	if !exists {
		return Decision{Rejection: &Rejection{
			StatusCode: 404,
			Message:    fmt.Sprintf("The model %q does not exist or is not available for this route", modelName),
			Type:       "invalid_request_error",
			Param:      "model",
			Code:       "model_not_found",
		}}
	}

	upstreamModel, err := json.Marshal(model.UpstreamModel)
	if err != nil {
		return Decision{Rejection: internalError()}
	}
	fields["model"] = upstreamModel
	body, err := json.Marshal(fields)
	if err != nil {
		return Decision{Rejection: internalError()}
	}
	return Decision{Mutation: Mutation{Body: body}}
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
	return internalError()
}

func requestFields(body []byte) (map[string]json.RawMessage, *Rejection) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, &Rejection{
			StatusCode: 400,
			Message:    "Request body must be a JSON object",
			Type:       "invalid_request_error",
			Code:       "invalid_json",
		}
	}

	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, &Rejection{
			StatusCode: 400,
			Message:    "Request body must be a valid JSON object",
			Type:       "invalid_request_error",
			Code:       "invalid_json",
		}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, &Rejection{
			StatusCode: 400,
			Message:    "Request body must contain one JSON object",
			Type:       "invalid_request_error",
			Code:       "invalid_json",
		}
	}
	return fields, nil
}

func invalidModel(message, code string) *Rejection {
	return &Rejection{
		StatusCode: 400,
		Message:    message,
		Type:       "invalid_request_error",
		Param:      "model",
		Code:       code,
	}
}

func internalError() *Rejection {
	return &Rejection{
		StatusCode: 500,
		Message:    "The request could not be processed",
		Type:       "server_error",
		Code:       "internal_error",
	}
}
