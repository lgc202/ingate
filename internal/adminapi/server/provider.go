package server

import "github.com/google/wire"

// ProviderSet 提供 Admin API 的 HTTP 协议注册与 transport
var ProviderSet = wire.NewSet(
	NewHTTPHandlers,
	NewHTTPServer,
)
