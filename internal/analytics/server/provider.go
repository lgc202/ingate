package server

import "github.com/google/wire"

// ProviderSet 汇总 Analytics 进程的所有 Kratos Server
var ProviderSet = wire.NewSet(
	NewHTTPServer,
	NewGRPCServer,
	NewConsumer,
)
