// Package service 实现 Envoy External Processing 协议。
//
// 同一客户端请求会产生一条 downstream 流和至少一条 upstream 流，服务只负责关联、
// 编排和生成 ExtProc 响应，具体 Chat Completions 协议转换位于 chatcompletion 子包。
package service

import "github.com/google/wire"

// ProviderSet 提供 Envoy External Processing 协议实现。
var ProviderSet = wire.NewSet(NewExternalProcessor, NewTokenQuotaUsageService)
