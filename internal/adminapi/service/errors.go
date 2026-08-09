// Package service 实现 Admin API 的传输协议适配
package service

// RequestError 表示 service 在请求自身中发现的错误
type RequestError struct {
	message string
}

// BadRequest 创建包含控制台提示的参数错误
func BadRequest(message string) error {
	return &RequestError{message: message}
}

func (e *RequestError) Error() string {
	return "invalid request"
}

// UserMessage 返回可以直接展示给控制台用户的提示
func (e *RequestError) UserMessage() string {
	return e.message
}
