package service

import "github.com/google/wire"

// ProviderSet 提供 Envoy External Processing 协议实现
var ProviderSet = wire.NewSet(NewExternalProcessor)
