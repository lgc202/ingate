package biz

import (
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
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

// NewUserError 使用 Kratos Error 表达可以直接展示给控制台用户的业务拒绝原因
func NewUserError(message string) error {
	return kratoserrors.New(500, adminv1.ErrorReason_BUSINESS_RULE_VIOLATION.String(), "request rejected").WithMetadata(map[string]string{
		"user_message": message,
	})
}
