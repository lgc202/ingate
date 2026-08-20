// Package service 实现 Admin API 的传输协议适配
package service

import (
	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

// BadRequest 创建符合 Admin API 错误契约的参数错误
func BadRequest(message string) error {
	return kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid request").
		WithMetadata(map[string]string{"user_message": message})
}
