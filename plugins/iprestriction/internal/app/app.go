// Package app 提供客户端 IP 访问限制插件的注册入口
package app

import pluginwasm "github.com/lgc202/ingate/plugins/iprestriction/internal/wasm"

// Register 注册客户端 IP 访问限制插件
func Register() {
	pluginwasm.Register()
}
