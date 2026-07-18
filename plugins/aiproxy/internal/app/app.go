// Package app 提供 AI Proxy 插件的注册入口
package app

import (
	"github.com/lgc202/ingate/plugins/aiproxy/internal/policy"
	pluginwasm "github.com/lgc202/ingate/plugins/aiproxy/internal/wasm"
)

// Register 注册 AI Proxy 插件
func Register() {
	pluginwasm.Register(policy.NewRunner())
}
