package auth

import "github.com/google/wire"

// ProviderSet 提供 Admin API 的身份认证能力
var ProviderSet = wire.NewSet(NewAuthenticator)
