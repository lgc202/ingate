// Package biz 定义 ingate-admin-api 的业务规则和数据访问边界。
package biz

import (
	"github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

var (
	// ErrResourceNotFound 表示声明式资源不存在。
	ErrResourceNotFound = errors.NotFound(
		adminv1.ErrorReason_RESOURCE_NOT_FOUND.String(),
		"资源不存在或已被删除",
	)
	// ErrResourceAlreadyExists 表示资源的持久化身份已经存在。
	ErrResourceAlreadyExists = errors.Conflict(
		adminv1.ErrorReason_RESOURCE_CONFLICT.String(),
		"资源已存在",
	)
	// ErrResourceVersionConflict 表示声明式资源已被其他请求修改。
	ErrResourceVersionConflict = errors.Conflict(
		adminv1.ErrorReason_RESOURCE_VERSION_CONFLICT.String(),
		"资源已被其他用户修改，请刷新后重试",
	)
	// ErrInvalidCursor 表示分页游标无法解析或已经失效。
	ErrInvalidCursor = errors.BadRequest(
		adminv1.ErrorReason_INVALID_ARGUMENT.String(),
		"分页游标无效或已过期",
	)
)

// NewInvalidResource 保留 API Server 的字段错误，同时只向控制台返回稳定提示。
func NewInvalidResource(cause error) error {
	return errors.BadRequest(
		adminv1.ErrorReason_INVALID_ARGUMENT.String(),
		"配置内容不正确",
	).WithCause(cause)
}
