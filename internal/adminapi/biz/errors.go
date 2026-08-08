package biz

import (
	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

// NewUserError 使用 Kratos Error 表达可以直接展示给控制台用户的业务拒绝原因
func NewUserError(message string) error {
	return kratoserrors.New(500, adminv1.ErrorReason_BUSINESS_RULE_VIOLATION.String(), "request rejected").WithMetadata(map[string]string{
		"user_message": message,
	})
}
