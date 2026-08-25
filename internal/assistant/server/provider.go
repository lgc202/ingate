package server

import "github.com/google/wire"

// ProviderSet 提供运维助手 HTTP transport 和后台 Run Worker。
var ProviderSet = wire.NewSet(NewStreamHandler, NewHTTPServer, NewRunWorker)
