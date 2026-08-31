// Package server 装配 Analytics 的 gRPC、HTTP 和 Kafka 请求记录消费循环。
package server

import "github.com/google/wire"

// ProviderSet 汇总 Analytics 进程的运维 HTTP、查询 gRPC 和请求记录消费循环。
var ProviderSet = wire.NewSet(
	NewHTTPServer,
	NewGRPCServer,
	NewRequestConsumer,
)
