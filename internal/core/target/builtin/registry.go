// Package builtin 提供项目内置 target 注册表
package builtin

import (
	"github.com/lgc202/ingate/internal/core/target"
	"github.com/lgc202/ingate/internal/core/target/debug"
	"github.com/lgc202/ingate/internal/core/target/xds"
)

// NewRegistry 创建包含内置 target 的注册表
func NewRegistry() (target.Registry, error) {
	return target.NewRegistry(debug.Translator{}, xds.Translator{})
}
