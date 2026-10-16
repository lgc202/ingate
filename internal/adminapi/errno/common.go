// Package errno 定义 Admin API 的公开协议错误，不承载业务或存储错误。
package errno

import (
	"context"
	"errors"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

var (
	// ErrInvalidArgument 表示请求结构不符合接口契约。
	ErrInvalidArgument = kratoserrors.BadRequest("INVALID_ARGUMENT", "请求参数不合法")
	// ErrRequestCanceled 表示请求已经取消。
	ErrRequestCanceled = kratoserrors.ClientClosed("REQUEST_CANCELED", "请求已取消")
	// ErrDeadlineExceeded 表示请求处理超过截止时间；保留已有公开 reason。
	ErrDeadlineExceeded = kratoserrors.GatewayTimeout("DEADLINE_EXCEEDED", "请求处理超时")
	// ErrInternal 隐藏未知内部错误的实现细节。
	ErrInternal = kratoserrors.InternalServer("INTERNAL_SERVER_ERROR", "服务器内部错误")
)

// Fallback 只公开取消、超时或固定的内部错误，不透传下游错误的消息和 metadata。
// WithCause 保留进程内原因；包级定义只读，不能修改其字段或 Metadata。
func Fallback(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		return ErrRequestCanceled.WithCause(err)
	case errors.Is(err, context.DeadlineExceeded):
		return ErrDeadlineExceeded.WithCause(err)
	default:
		return ErrInternal.WithCause(err)
	}
}
