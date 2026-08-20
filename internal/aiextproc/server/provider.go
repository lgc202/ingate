package server

import "github.com/google/wire"

// ProviderSet 汇总 AI ExtProc 的运维 HTTP 和 External Processing gRPC 服务
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer)
