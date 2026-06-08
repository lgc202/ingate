// Package app 提供 ACL 插件的注册入口
package app

import (
	"github.com/lgc202/ingate/plugins/acl/internal/policy"
	pluginwasm "github.com/lgc202/ingate/plugins/acl/internal/wasm"
)

// Register 注册 ACL 插件
func Register() {
	pluginwasm.Register(policy.NewRunner())
}
