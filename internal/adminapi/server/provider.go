package server

import "github.com/google/wire"

// ProviderSet 提供 Admin API 的协议服务及 HTTP、gRPC transport。
var ProviderSet = wire.NewSet(
	NewServices,
	NewHTTPServer,
	NewGRPCServer,
)
