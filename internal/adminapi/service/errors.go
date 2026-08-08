package service

import (
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/biz"
)

const userMessageMetadata = "user_message"

// BadRequest 创建包含控制台提示的参数错误
func BadRequest(message string) error {
	return kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid request").WithMetadata(map[string]string{
		userMessageMetadata: message,
	})
}

// OperationError 把领域错误转换为稳定的 Admin API 错误
func OperationError(err error, message string) error {
	if err == nil {
		return nil
	}
	var userError *biz.UserError
	if errors.As(err, &userError) {
		return kratoserrors.New(500, adminv1.ErrorReason_BUSINESS_RULE_VIOLATION.String(), "request rejected").
			WithMetadata(map[string]string{userMessageMetadata: userError.UserMessage()}).
			WithCause(err)
	}
	var serviceError *kratoserrors.Error
	if errors.As(err, &serviceError) {
		return err
	}
	return kratoserrors.InternalServer(adminv1.ErrorReason_INTERNAL_ERROR.String(), "operation failed").
		WithMetadata(map[string]string{userMessageMetadata: message}).
		WithCause(err)
}
