package auth

import "context"

type principalKey struct{}

// NewContext 把已验证身份放入请求上下文，供授权、审计和业务操作读取
func NewContext(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

// FromContext 返回当前请求的已验证身份
func FromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}
