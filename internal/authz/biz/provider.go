package biz

import (
	"github.com/google/wire"

	"github.com/lgc202/ingate/internal/authz/biz/ratelimit"
)

// ProviderSet 提供 Caller 授权和请求限流业务能力
var ProviderSet = wire.NewSet(NewAuthorizer, ratelimit.NewService)
