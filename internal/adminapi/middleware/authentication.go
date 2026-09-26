package middleware

import (
	"context"
	"errors"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	adminhandler "github.com/lgc202/ingate/internal/adminapi/handler"
	accountservice "github.com/lgc202/ingate/internal/adminapi/service/account"
)

const authorizationHeader = "Authorization"

type sessionKey struct{}

// RequireSession 校验 Bearer Token，并将账户身份放入请求 Context。
func RequireSession(service *accountservice.Service) kratosmiddleware.Middleware {
	return func(next kratosmiddleware.Handler) kratosmiddleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			serverTransport, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, adminhandler.ToTransportError(ctx, errors.New("server transport not found"))
			}

			token, ok := bearerToken(serverTransport.RequestHeader().Get(authorizationHeader))
			if !ok {
				return nil, ToAuthenticationError(ctx, accountservice.ErrSessionRequired)
			}

			session, err := service.Authenticate(ctx, token)
			if err != nil {
				return nil, ToAuthenticationError(ctx, err)
			}

			return next(context.WithValue(ctx, sessionKey{}, session), req)
		}
	}
}

// SessionFromContext 返回已认证的账户身份。
func SessionFromContext(ctx context.Context) (accountservice.AuthenticatedSession, bool) {
	session, ok := ctx.Value(sessionKey{}).(accountservice.AuthenticatedSession)
	return session, ok
}

// ToAuthenticationError 将会话错误转换为公开协议错误。
func ToAuthenticationError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return adminhandler.ToTransportError(ctx, err)
	case errors.Is(err, accountservice.ErrSessionRequired):
		return kratoserrors.Unauthorized("AUTH_SESSION_REQUIRED", "请先登录").WithCause(err)
	case errors.Is(err, accountservice.ErrUnavailable):
		return kratoserrors.ServiceUnavailable("AUTH_UNAVAILABLE", "认证服务暂时不可用").WithCause(err)
	default:
		return adminhandler.ToTransportError(ctx, err)
	}
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	return parts[1], true
}
