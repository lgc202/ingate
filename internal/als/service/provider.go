package service

import "github.com/google/wire"

// ProviderSet 汇总 Envoy ALS 协议实现
var ProviderSet = wire.NewSet(NewService)
