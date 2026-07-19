package openai

import (
	"encoding/json"
	"fmt"
)

// ErrorDetail 表示 OpenAI-compatible 错误详情
type ErrorDetail struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code,omitempty"`
}

// ErrorEnvelope 表示 OpenAI-compatible 错误响应体
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// DefaultError 根据 HTTP 状态码构造不泄漏内部实现的上游错误
func DefaultError(statusCode int, message string) ErrorDetail {
	if message == "" {
		message = "upstream request failed"
	}
	detail := ErrorDetail{
		Message: message,
		Type:    errorType(statusCode),
	}
	if statusCode > 0 {
		detail.Code = fmt.Sprintf("%d", statusCode)
	}
	return detail
}

// EncodeError 编码 OpenAI-compatible 错误响应体
func EncodeError(detail ErrorDetail) []byte {
	// ErrorDetail 只包含 JSON 原生字段，编码不会失败
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
