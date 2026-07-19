package llm

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrInvalidRequest 表示客户端请求不符合第一阶段文本 Chat 协议
	ErrInvalidRequest = errors.New("invalid chat completion request")
	// ErrUnsupportedFeature 表示请求或响应使用了第一阶段未支持的能力
	ErrUnsupportedFeature = errors.New("unsupported chat completion feature")
	// ErrInvalidResponse 表示上游普通响应无法转换
	ErrInvalidResponse = errors.New("invalid upstream chat completion response")
	// ErrInvalidStream 表示上游 SSE 事件或状态序列无法转换
	ErrInvalidStream = errors.New("invalid upstream chat completion stream")
)

// APIError 表示 OpenAI-compatible 错误详情
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ErrorEnvelope 表示 OpenAI-compatible 错误响应体
type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

// DefaultAPIError 根据 HTTP 状态码构造不泄漏内部实现的上游错误
func DefaultAPIError(statusCode int, message string) APIError {
	if message == "" {
		message = "upstream request failed"
	}
	detail := APIError{
		Message: message,
		Type:    errorType(statusCode),
	}
	if statusCode > 0 {
		detail.Code = fmt.Sprintf("%d", statusCode)
	}
	return detail
}

// EncodeError 编码 OpenAI-compatible 错误响应体
func EncodeError(detail APIError) []byte {
	// APIError 只包含 JSON 原生字符串字段，编码不会失败
	body, _ := json.Marshal(ErrorEnvelope{Error: detail})
	return body
}

func errorType(statusCode int) string {
	switch statusCode {
	case 400:
		return "invalid_request_error"
	case 401:
		return "authentication_error"
	case 403:
		return "permission_error"
	case 404:
		return "not_found_error"
	case 409:
		return "conflict_error"
	case 429:
		return "rate_limit_error"
	case 500, 502, 503, 504:
		return "server_error"
	default:
		return "api_error"
	}
}
