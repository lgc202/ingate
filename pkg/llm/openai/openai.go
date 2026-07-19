package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lgc202/ingate/pkg/llm"
)

// ChatCompletionsPath 是相对于 OpenAI API Base Path 的请求路径
const ChatCompletionsPath = "/chat/completions"

// TransformRequest 校验文本 Chat 请求并只替换 model 字段，其余 JSON 字段保持不变
func TransformRequest(body []byte, upstreamModel string) ([]byte, error) {
	if strings.TrimSpace(upstreamModel) == "" {
		return nil, fmt.Errorf("%w: upstream model must not be empty", llm.ErrInvalidRequest)
	}
	original, err := llm.DecodeChatRequest(body)
	if err != nil {
		return nil, err
	}
	if err := original.ValidateSupported(); err != nil {
		return nil, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("%w: decode request body: %v", llm.ErrInvalidRequest, err)
	}
	model, err := json.Marshal(upstreamModel)
	if err != nil {
		return nil, fmt.Errorf("encode upstream model: %w", err)
	}
	fields["model"] = model
	transformed, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI request: %w", err)
	}
	return transformed, nil
}

// TransformResponse 把成功响应中的上游模型名替换为客户端公开别名
func TransformResponse(body []byte, publicModel string) ([]byte, error) {
	if strings.TrimSpace(publicModel) == "" {
		return nil, fmt.Errorf("%w: public model must not be empty", llm.ErrInvalidResponse)
	}
	if !json.Valid(body) || len(bytes.TrimSpace(body)) == 0 || bytes.TrimSpace(body)[0] != '{' {
		return nil, fmt.Errorf("%w: response body must be a JSON object", llm.ErrInvalidResponse)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("%w: decode OpenAI response: %v", llm.ErrInvalidResponse, err)
	}
	model, err := json.Marshal(publicModel)
	if err != nil {
		return nil, fmt.Errorf("encode public model: %w", err)
	}
	fields["model"] = model
	transformed, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI response: %w", err)
	}
	return transformed, nil
}

// TransformError 把 OpenAI-compatible 上游错误规范化为稳定的错误信封
func TransformError(body []byte, statusCode int) []byte {
	type upstreamError struct {
		Message string          `json:"message"`
		Type    string          `json:"type"`
		Param   string          `json:"param"`
		Code    json.RawMessage `json:"code"`
	}
	var envelope struct {
		Error upstreamError `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error.Message == "" {
		return llm.EncodeError(llm.DefaultAPIError(statusCode, "upstream returned an invalid error response"))
	}

	detail := llm.APIError{
		Message: envelope.Error.Message,
		Type:    envelope.Error.Type,
		Param:   envelope.Error.Param,
		Code:    decodeCode(envelope.Error.Code),
	}
	if detail.Type == "" {
		detail.Type = llm.DefaultAPIError(statusCode, "").Type
	}
	return llm.EncodeError(detail)
}

func decodeCode(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return number.String()
	}
	return string(raw)
}
