package service

import (
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
)

const userMessageMetadata = "user_message"

func badRequest(message string) error {
	return kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid request").WithMetadata(map[string]string{
		userMessageMetadata: message,
	})
}

func operationError(err error, message string) error {
	if err == nil {
		return nil
	}
	var serviceError *kratoserrors.Error
	if errors.As(err, &serviceError) {
		return err
	}
	return kratoserrors.InternalServer(adminv1.ErrorReason_INTERNAL_ERROR.String(), "operation failed").
		WithMetadata(map[string]string{userMessageMetadata: message}).
		WithCause(err)
}
