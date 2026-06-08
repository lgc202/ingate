// Package acl 定义内置 ACL 插件的可执行配置
//
// 这个包位于控制面 target 和 Wasm 插件之间，描述 xDS 下发给插件的运行时配置。
// 它不是控制台资源模型；后续控制面资源可以编译成这里的 route 执行规则。
package acl
