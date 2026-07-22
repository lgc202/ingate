// Package app 提供 TokenQuota 插件的注册入口
package app

import pluginwasm "github.com/lgc202/ingate/plugins/tokenquota/internal/wasm"

// Register 注册 TokenQuota 插件
func Register() {
	pluginwasm.Register()
}
