// Package target 定义运行时 target 翻译边界
package target

import (
	"github.com/lgc202/ingate-next/internal/core/ir"
	"github.com/lgc202/ingate-next/internal/core/runtime"
)

// Translator 表示一个运行时 target 的翻译器
type Translator interface {
	Target() string
	Translate(logical ir.LogicalGateway) (runtime.RuntimeSnapshot, error)
}
