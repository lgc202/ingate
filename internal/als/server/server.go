// Package server 装配 ingate-als 的 Kratos transport 和积压回放任务。
package server

import "github.com/google/wire"

// ProviderSet 汇总 ALS 进程的运维 HTTP、Envoy gRPC 和磁盘队列回放任务。
var ProviderSet = wire.NewSet(
	NewHTTPServer,
	NewGRPCServer,
	NewDiskQueueReplayer,
)
