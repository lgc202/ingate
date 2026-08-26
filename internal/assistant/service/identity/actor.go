// Package identity 从管理面代理转发的请求中读取调用者身份。
package identity

import (
	"context"
	"strings"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
)

const (
	ForwardedUserHeader = "X-Forwarded-User"
	maxActorIDLength    = 128
)

// ActorID 读取 Admin API 在内部请求上传递的用户标识。
// Assistant 不自行解析登录凭据，但所有用户数据都必须按该标识隔离。
func ActorID(ctx context.Context) (string, error) {
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", kratoserrors.Unauthorized("ACTOR_REQUIRED", "authentication required")
	}
	return ValidateActorID(tr.RequestHeader().Get(ForwardedUserHeader))
}

// ValidateActorID 验证从自定义 HTTP 路由读取的用户标识。
func ValidateActorID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", kratoserrors.Unauthorized("ACTOR_REQUIRED", "authentication required")
	}
	if len(value) > maxActorIDLength {
		return "", kratoserrors.BadRequest("INVALID_ARGUMENT", "actor identifier is too long")
	}
	return value, nil
}
