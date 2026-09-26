package server

import (
	"context"

	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	adminmiddleware "github.com/lgc202/ingate/internal/adminapi/middleware"
)

// MiddlewareChain 保存 HTTP 和 gRPC 共用的有序入站中间件链。
type MiddlewareChain struct {
	middlewares []kratosmiddleware.Middleware
}

// NewMiddlewareChain 构造 Admin API 的完整入站中间件链。
func NewMiddlewareChain(authentication kratosmiddleware.Middleware) MiddlewareChain {
	return MiddlewareChain{
		middlewares: []kratosmiddleware.Middleware{
			selector.Server(authentication).Match(requiresAuthentication).Build(),
			adminmiddleware.ValidateRequests(),
		},
	}
}

func requiresAuthentication(_ context.Context, operation string) bool {
	return operation != adminv1.OperationAccountServiceLogin
}
