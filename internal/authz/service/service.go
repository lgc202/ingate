// Package service 适配 Envoy External Authorization 协议。
package service

import "github.com/google/wire"

// ProviderSet 提供 Envoy External Authorization 协议实现。
var ProviderSet = wire.NewSet(NewAuthorizationService)
