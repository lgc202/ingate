// Package biz 定义 ingate-admin-api 的业务规则和数据访问边界
package biz

import (
	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

var (
	// ErrResourceNotFound 表示声明式资源不存在
	ErrResourceNotFound = kratoserrors.NotFound(adminv1.ErrorReason_RESOURCE_NOT_FOUND.String(), "resource not found").
				WithMetadata(map[string]string{"user_message": "资源不存在或已被删除"})
	// ErrResourceVersionConflict 表示声明式资源已被其他请求修改
	ErrResourceVersionConflict = kratoserrors.Conflict(adminv1.ErrorReason_RESOURCE_VERSION_CONFLICT.String(), "resource version conflict").
					WithMetadata(map[string]string{"user_message": "资源已被其他用户修改，请刷新后重试"})
	// ErrInvalidCursor 表示分页游标无法解析或已经失效
	ErrInvalidCursor = kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid cursor").
				WithMetadata(map[string]string{"user_message": "分页游标无效或已过期"})
)

// NewUserError 创建可展示的业务错误
func NewUserError(message string) error {
	return kratoserrors.Conflict(adminv1.ErrorReason_BUSINESS_RULE_VIOLATION.String(), "request rejected").
		WithMetadata(map[string]string{"user_message": message})
}

// NewVersionConflictError 创建可以向用户说明的乐观锁冲突
func NewVersionConflictError(resourceID, userMessage string) error {
	return kratoserrors.Conflict(adminv1.ErrorReason_RESOURCE_VERSION_CONFLICT.String(), "resource version conflict").
		WithMetadata(map[string]string{
			"resource_id":  resourceID,
			"user_message": userMessage,
		})
}
