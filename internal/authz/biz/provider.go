package biz

import "github.com/google/wire"

// ProviderSet 提供 Caller 授权业务能力
var ProviderSet = wire.NewSet(NewAuthorizer)
