// Package server 装配运维助手的 HTTP 与 Temporal Worker 运行入口。
package server

import "github.com/google/wire"

// ProviderSet 提供运维助手的进程入口。
var ProviderSet = wire.NewSet(NewHTTPServer, NewWorker)
