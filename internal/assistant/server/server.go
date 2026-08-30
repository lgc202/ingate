// Package server 装配运维助手的 Kratos HTTP transport。
package server

import "github.com/google/wire"

// ProviderSet 提供运维助手的 HTTP 服务和后台执行消费者。
var ProviderSet = wire.NewSet(NewStreamHandler, NewHTTPServer, NewExecutionConsumer)
