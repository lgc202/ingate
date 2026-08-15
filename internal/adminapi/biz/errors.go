// Package biz 定义 ingate-admin-api 的业务规则和数据访问边界
package biz

import (
	"errors"
	"fmt"
	"log/slog"
)

var (
	// ErrResourceNotFound 表示声明式资源不存在
	ErrResourceNotFound = errors.New("resource not found")
	// ErrResourceVersionConflict 表示声明式资源已被其他请求修改
	ErrResourceVersionConflict = errors.New("resource version conflict")
	// ErrInvalidCursor 表示分页游标无法解析或已经失效
	ErrInvalidCursor = errors.New("invalid cursor")
)

// UserError 表示可以向控制台用户说明的业务拒绝，不包含传输协议语义
type UserError struct {
	message string
}

// VersionConflictError 表示用户提交的配置版本已经过期
type VersionConflictError struct {
	resourceID  string
	userMessage string
}

// NewUserError 创建可展示的业务错误
func NewUserError(message string) error {
	return &UserError{message: message}
}

// NewVersionConflictError 创建可以向用户说明的乐观锁冲突
func NewVersionConflictError(resourceID, userMessage string) error {
	return &VersionConflictError{resourceID: resourceID, userMessage: userMessage}
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

// Error 返回不包含用户展示文案的稳定错误语义
func (e *VersionConflictError) Error() string {
	return fmt.Sprintf("resource version conflict: %s", e.resourceID)
}

// Unwrap 支持调用方使用 errors.Is 判断版本冲突
func (e *VersionConflictError) Unwrap() error {
	return ErrResourceVersionConflict
}

// UserMessage 返回可以直接展示给控制台用户的冲突提示
func (e *VersionConflictError) UserMessage() string {
	return e.userMessage
}

// LogValue 只记录资源 ID，不把中文提示写入结构化日志
func (e *VersionConflictError) LogValue() slog.Value {
	return slog.StringValue(e.Error())
}
