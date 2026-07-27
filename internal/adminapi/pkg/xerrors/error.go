package xerrors

import "log/slog"

// UserError 表示错误消息可以直接展示给用户
type UserError struct {
	message string
}

// NewUserError 创建可展示给用户的错误
func NewUserError(message string) *UserError {
	return &UserError{message: message}
}

// Error 返回可以直接用于前端提示的错误文案
func (e *UserError) Error() string {
	return e.message
}

// LogValue 避免面向用户的中文错误正文进入结构化日志
func (e *UserError) LogValue() slog.Value {
	return slog.StringValue("request rejected")
}
