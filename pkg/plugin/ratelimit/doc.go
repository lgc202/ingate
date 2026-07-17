// Package ratelimit 定义内置 RateLimit 插件的可执行配置
//
// 这个包位于控制面 target 和 Wasm 插件之间，描述 xDS 下发给插件的运行时配置。
// 它不是控制台资源模型，也不包含系统 Redis 的连接信息。
package ratelimit
