package handler

import (
	"context"
	"errors"
	"log/slog"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// ToInvalidArgument 将协议校验失败转换为公开错误。
func ToInvalidArgument(err error) error {
	return kratoserrors.BadRequest("INVALID_ARGUMENT", "请求参数不合法").WithCause(err)
}

// ToTransportError 将非业务错误转换为公开协议错误。
func ToTransportError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return kratoserrors.ClientClosed("REQUEST_CANCELED", "请求已取消").WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return kratoserrors.GatewayTimeout("DEADLINE_EXCEEDED", "请求处理超时").WithCause(err)
	default:
		slog.ErrorContext(ctx, "Admin API request failed", "err", err)
		return kratoserrors.InternalServer("INTERNAL_SERVER_ERROR", "服务器内部错误").WithCause(err)
	}
}
