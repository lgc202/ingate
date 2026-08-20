package server

import "github.com/google/wire"

// ProviderSet 汇总 Authz 的运维 HTTP 和 External Authorization gRPC 服务
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer)
