package service

import "github.com/google/wire"

// ProviderSet 提供 Envoy External Authorization 协议实现
var ProviderSet = wire.NewSet(NewAuthorizationService)
