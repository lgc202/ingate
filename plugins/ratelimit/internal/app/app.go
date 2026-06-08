// Package app 提供 RateLimit 插件的注册入口
package app

import (
	"github.com/lgc202/ingate/plugins/ratelimit/internal/policy"
	pluginwasm "github.com/lgc202/ingate/plugins/ratelimit/internal/wasm"
)

// Register 注册 RateLimit 插件
func Register() {
	pluginwasm.Register(policy.NewSharedDataRunner())
}
