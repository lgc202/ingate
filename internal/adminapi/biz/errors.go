// Package biz 实现 ingate-admin-api 的业务用例和数据访问边界
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
	// ErrAccessKeyNotFound 表示访问密钥不存在
	ErrAccessKeyNotFound = errors.New("access key not found")
	// ErrAccessKeyNameConflict 表示访问密钥名称违反唯一约束
	ErrAccessKeyNameConflict = errors.New("access key name already exists")
	// ErrInvalidPageToken 表示分页游标无法解析或已经失效
	ErrInvalidPageToken = errors.New("invalid page token")
	// ErrDisplayNameConflict 表示同类声明式资源已经使用该展示名称
	ErrDisplayNameConflict = errors.New("display name conflict")
)

// UserError 表示可以向控制台用户说明的业务拒绝，不包含传输协议语义
type UserError struct {
	message string
}

// NewUserError 创建可展示的业务错误
func NewUserError(message string) error {
	return &UserError{message: message}
}

// DisplayNameConflict 把 API Server 的唯一性裁决转换为控制台可展示的业务提示
func DisplayNameConflict(err error, resourceLabel, displayName string) error {
	if errors.Is(err, ErrDisplayNameConflict) {
		return NewUserError(fmt.Sprintf("%s名称 %q 已存在", resourceLabel, displayName))
	}
	return err
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
