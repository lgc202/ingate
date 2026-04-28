// Package pipeline 串联资源编译和 target 翻译
package pipeline

import (
	"fmt"

	"github.com/lgc202/ingate-next/internal/core/compiler"
	"github.com/lgc202/ingate-next/internal/core/resource"
	"github.com/lgc202/ingate-next/internal/core/runtime"
	"github.com/lgc202/ingate-next/internal/core/target"
)

// Pipeline 表示控制面核心编译流水线
type Pipeline struct {
	Compiler   compiler.Compiler
	Translator target.Translator
}

// BuildGatewaySnapshot 将资源集合编译成指定 Gateway 的运行时快照
func (p Pipeline) BuildGatewaySnapshot(bundle resource.Bundle, gatewayName string) (runtime.RuntimeSnapshot, error) {
	logical, err := p.Compiler.CompileGateway(bundle, gatewayName)
	if err != nil {
		return runtime.RuntimeSnapshot{}, fmt.Errorf("compile gateway %q: %w", gatewayName, err)
	}

	snapshot, err := p.Translator.Translate(logical)
	if err != nil {
		return runtime.RuntimeSnapshot{}, fmt.Errorf("translate gateway %q to target %q: %w", gatewayName, p.Translator.Target(), err)
	}

	return snapshot, nil
}
