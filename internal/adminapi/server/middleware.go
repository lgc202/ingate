package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"buf.build/go/protovalidate"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"google.golang.org/protobuf/proto"

	adminv1 "github.com/lgc202/ingate/api/admin/v1"
	"github.com/lgc202/ingate/internal/adminapi/auth"
	"github.com/lgc202/ingate/internal/adminapi/biz"
	requestbiz "github.com/lgc202/ingate/internal/adminapi/biz/request"
	adminservice "github.com/lgc202/ingate/internal/adminapi/service"
	"github.com/lgc202/ingate/internal/pkg/requestid"
)

// httpMiddleware 按请求从外到内的执行顺序装配管理面中间件
func httpMiddleware(logger *slog.Logger, authenticator *auth.Authenticator) []middleware.Middleware {
	return []middleware.Middleware{
		recoveryMiddleware(logger),
		requestLoggingMiddleware(logger),
		authenticationMiddleware(authenticator),
		auditMiddleware(logger),
		authorizationMiddleware,
		errorMappingMiddleware,
		requestValidationMiddleware,
	}
}

func requestValidationMiddleware(next middleware.Handler) middleware.Handler {
	return func(ctx context.Context, request any) (any, error) {
		message, ok := request.(proto.Message)
		if !ok {
			return next(ctx, request)
		}
		if err := protovalidate.Validate(message); err != nil {
			return nil, kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid request").
				WithMetadata(map[string]string{userMessageMetadata: "请求参数不正确"}).
				WithCause(err)
		}
		return next(ctx, request)
	}
}

func errorMappingMiddleware(next middleware.Handler) middleware.Handler {
	return func(ctx context.Context, request any) (any, error) {
		reply, err := next(ctx, request)
		if err == nil {
			return reply, nil
		}

		if requestError, ok := errors.AsType[*adminservice.RequestError](err); ok {
			return nil, kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid request").
				WithMetadata(map[string]string{userMessageMetadata: requestError.UserMessage()}).
				WithCause(err)
		}
		if conflictError, ok := errors.AsType[*biz.VersionConflictError](err); ok {
			return nil, kratoserrors.New(http.StatusConflict, adminv1.ErrorReason_RESOURCE_VERSION_CONFLICT.String(), "resource version conflict").
				WithMetadata(map[string]string{userMessageMetadata: conflictError.UserMessage()}).
				WithCause(err)
		}
		if userError, ok := errors.AsType[*biz.UserError](err); ok {
			return nil, kratoserrors.New(http.StatusConflict, adminv1.ErrorReason_BUSINESS_RULE_VIOLATION.String(), "request rejected").
				WithMetadata(map[string]string{userMessageMetadata: userError.UserMessage()}).
				WithCause(err)
		}
		if errors.Is(err, biz.ErrResourceNotFound) {
			return nil, kratoserrors.New(http.StatusNotFound, adminv1.ErrorReason_RESOURCE_NOT_FOUND.String(), "resource not found").
				WithMetadata(map[string]string{userMessageMetadata: "资源不存在或已被删除"}).
				WithCause(err)
		}
		if errors.Is(err, requestbiz.ErrNotFound) {
			return nil, kratoserrors.New(http.StatusNotFound, adminv1.ErrorReason_REQUEST_RECORD_NOT_FOUND.String(), "request record not found").
				WithMetadata(map[string]string{userMessageMetadata: "请求记录不存在或已超过明细保留期"}).
				WithCause(err)
		}
		if errors.Is(err, requestbiz.ErrUnavailable) {
			return nil, kratoserrors.New(http.StatusServiceUnavailable, adminv1.ErrorReason_DEPENDENCY_UNAVAILABLE.String(), "request analytics unavailable").
				WithMetadata(map[string]string{userMessageMetadata: "请求记录服务暂时不可用，请稍后重试"}).
				WithCause(err)
		}
		if errors.Is(err, biz.ErrInvalidCursor) {
			return nil, kratoserrors.BadRequest(adminv1.ErrorReason_INVALID_ARGUMENT.String(), "invalid cursor").
				WithMetadata(map[string]string{userMessageMetadata: "分页游标无效或已过期"}).
				WithCause(err)
		}
		if _, ok := errors.AsType[*kratoserrors.Error](err); ok {
			return nil, err
		}
		return nil, kratoserrors.InternalServer(adminv1.ErrorReason_INTERNAL_ERROR.String(), "operation failed").
			WithMetadata(map[string]string{userMessageMetadata: "请求处理失败"}).
			WithCause(err)
	}
}

func authenticationMiddleware(authenticator *auth.Authenticator) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			operation := serverOperation(ctx)
			if auth.IsPublicOperation(operation) {
				return next(ctx, request)
			}
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, kratoserrors.Unauthorized(adminv1.ErrorReason_UNAUTHENTICATED.String(), "authentication required").
					WithMetadata(map[string]string{userMessageMetadata: "请先登录"})
			}
			principal, err := authenticator.Authenticate(ctx, tr.RequestHeader().Get("Authorization"))
			if err != nil {
				return nil, kratoserrors.Unauthorized(adminv1.ErrorReason_UNAUTHENTICATED.String(), "authentication failed").
					WithMetadata(map[string]string{userMessageMetadata: "登录状态已失效，请重新登录"}).
					WithCause(err)
			}
			return next(auth.NewContext(ctx, principal), request)
		}
	}
}

func authorizationMiddleware(next middleware.Handler) middleware.Handler {
	return func(ctx context.Context, request any) (any, error) {
		operation := serverOperation(ctx)
		if auth.IsPublicOperation(operation) {
			return next(ctx, request)
		}
		principal, ok := auth.FromContext(ctx)
		if !ok || !auth.Allowed(principal.Role, operation) {
			return nil, kratoserrors.Forbidden(adminv1.ErrorReason_PERMISSION_DENIED.String(), "permission denied").
				WithMetadata(map[string]string{userMessageMetadata: "当前账号没有执行此操作的权限"})
		}
		return next(ctx, request)
	}
}

func requestIDFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		id := request.Header.Get(requestid.Header)
		if id == "" {
			id = requestid.New()
			request.Header.Set(requestid.Header, id)
		}
		writer.Header().Set(requestid.Header, id)
		next.ServeHTTP(writer, request)
	})
}

func recoveryMiddleware(logger *slog.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (reply any, err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.ErrorContext(ctx, "panic recovered",
						"panic", recovered,
						"stack", string(debug.Stack()),
					)
					err = kratoserrors.InternalServer(adminv1.ErrorReason_PANIC.String(), "request failed").
						WithMetadata(map[string]string{userMessageMetadata: "请求处理失败"})
				}
			}()
			return next(ctx, request)
		}
	}
}

func requestLoggingMiddleware(logger *slog.Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, request any) (any, error) {
			startedAt := time.Now()
			reply, err := next(ctx, request)
			code := http.StatusOK
			reason := ""
			operation := ""
			id := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation = tr.Operation()
				id = tr.RequestHeader().Get(requestid.Header)
			}
			if serviceError := kratoserrors.FromError(err); serviceError != nil {
				code = int(serviceError.Code)
				reason = serviceError.Reason
			}
			if code == http.StatusBadRequest && reason == adminv1.ErrorReason_INVALID_ARGUMENT.String() {
				// 请求自身可判断的参数错误直接返回，不污染服务日志
				return reply, err
			}
			attrs := []any{
				"operation", operation,
				"code", code,
				"reason", reason,
				"latency", time.Since(startedAt),
				"request_id", id,
			}
			if code >= http.StatusInternalServerError {
				if cause := errors.Unwrap(err); cause != nil {
					attrs = append(attrs, "error", cause)
				}
				logger.ErrorContext(ctx, "server request", attrs...)
			} else {
				// 参数错误属于正常请求结果，不按服务异常记录
				logger.InfoContext(ctx, "server request", attrs...)
			}
			return reply, err
		}
	}
}

func serverOperation(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}
