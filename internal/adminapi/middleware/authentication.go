package middleware

import (
	"context"
	"errors"
	"strings"

	kratosmiddleware "github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/lgc202/ingate/internal/adminapi/errno"
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
				return nil, errno.Fallback(errors.New("server transport not found"))
			}

			token, ok := bearerToken(serverTransport.RequestHeader().Get(authorizationHeader))
			if !ok {
				return nil, toAuthenticationError(accountservice.ErrSessionRequired)
			}

			session, err := service.Authenticate(ctx, token)
			if err != nil {
				return nil, toAuthenticationError(err)
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

func toAuthenticationError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return errno.Fallback(err)
	case errors.Is(err, accountservice.ErrSessionRequired):
		return errno.ErrAuthSessionRequired.WithCause(err)
	case errors.Is(err, accountservice.ErrUnavailable):
		return errno.ErrAuthUnavailable.WithCause(err)
	default:
		return errno.Fallback(err)
	}
}

func bearerToken(authorization string) (string, bool) {
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	return parts[1], true
}
