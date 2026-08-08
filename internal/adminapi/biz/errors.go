package biz

import (
	"errors"
	"log/slog"
)

var (
	// ErrResourceNotFound 表示声明式资源不存在
	ErrResourceNotFound = errors.New("resource not found")
	// ErrResourceVersionConflict 表示声明式资源已被其他请求修改
	ErrResourceVersionConflict = errors.New("resource version conflict")
	// ErrAccessKeyNotFound 表示访问密钥不存在
	ErrAccessKeyNotFound = errors.New("access key not found")
	// ErrAccessKeyNameConflict 表示访问密钥名称违反唯一约束
	ErrAccessKeyNameConflict = errors.New("access key name already exists")
)

// UserError 表示可以向控制台用户说明的业务拒绝，不包含传输协议语义
type UserError struct {
	message string
}

// NewUserError 创建可展示的业务错误
func NewUserError(message string) error {
	return &UserError{message: message}
}

// Error 返回业务拒绝的真实说明
func (e *UserError) Error() string {
	return e.message
}

// UserMessage 返回可以直接展示给控制台用户的错误说明
func (e *UserError) UserMessage() string {
	return e.message
}

// LogValue 防止用户提示进入结构化日志，只保留稳定的英文错误语义
func (e *UserError) LogValue() slog.Value {
	return slog.StringValue("business rule violation")
}
