package middleware

import (
	"context"
	"errors"
	"fmt"

	"buf.build/go/protovalidate"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	"google.golang.org/protobuf/proto"
)

// ValidateRequests 在请求进入 Handler 前执行 protobuf 契约校验。
func ValidateRequests() kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			message, ok := req.(proto.Message)
			if !ok {
				return next(ctx, req)
			}

			if err := protovalidate.Validate(message); err != nil {
				if _, ok := errors.AsType[*protovalidate.ValidationError](err); ok {
					return nil, adminhandler.ToInvalidArgument(err)
				}
				return nil, adminhandler.ToTransportError(ctx, fmt.Errorf("validate request contract: %w", err))
			}

			return next(ctx, req)
		}
	}
}
