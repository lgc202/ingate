package server

import "github.com/google/wire"

// ProviderSet 提供运维助手 HTTP transport。
var ProviderSet = wire.NewSet(NewHTTPServer)
