// Package biz 实现调用方身份认证和 Route 访问授权。
package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/authz/biz/ratelimit"
)

// ProviderSet 提供 Caller 授权和请求限流业务能力。
var ProviderSet = wire.NewSet(NewAuthorizer, ratelimit.NewLimiter)
