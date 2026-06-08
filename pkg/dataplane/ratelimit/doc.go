// Package ratelimit 定义限流插件与 ingate-dataplane 之间的稳定数据契约
//
// 这里的类型只描述跨进程请求和响应，不承载 Redis client、限流算法或插件执行逻辑。
// 插件和 ingate-dataplane 共同依赖这个包，避免两边各自维护一份 JSON 结构。
// HTTP 路由、传输方式和具体执行逻辑分别由调用方自己的 handler、client 和 service 负责。
package ratelimit
