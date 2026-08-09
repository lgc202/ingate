package server

import (
	"context"
	"log/slog"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"

	"github.com/lgc202/ingate/internal/adminapi/auth"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

type resourceIdentifier interface {
	GetId() string
}

func auditMiddleware(logger *slog.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			operation := serverOperation(ctx)
			if !auth.IsWriteOperation(operation) {
				return next(ctx, request)
			}

			startedAt := time.Now()
			reply, err := next(ctx, request)
			principal, _ := auth.FromContext(ctx)
			resourceID := identifier(request)
			if resourceID == "" {
				resourceID = identifier(reply)
			}
			requestID := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				requestID = tr.RequestHeader().Get(requestid.Header)
			}
			reason := ""
			if serviceError := kratoserrors.FromError(err); serviceError != nil {
				reason = serviceError.Reason
			}
			logger.InfoContext(ctx, "admin audit event",
				"event_kind", "audit",
				"actor_subject", principal.Subject,
				"actor_name", principal.Name,
				"actor_email", principal.Email,
				"actor_role", principal.Role,
				"operation", operation,
				"resource_id", resourceID,
				"success", err == nil,
				"reason", reason,
				"request_id", requestID,
				"latency", time.Since(startedAt),
			)
			return reply, err
		}
	}
}

func identifier(value any) string {
	identified, ok := value.(resourceIdentifier)
	if !ok {
		return ""
	}
	return identified.GetId()
}
