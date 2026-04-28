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
	Registry   target.Registry
}

// BuildGatewaySnapshot 将资源集合编译成指定 Gateway 的运行时快照
func (p Pipeline) BuildGatewaySnapshot(bundle resource.Bundle, gatewayName string) (runtime.RuntimeSnapshot, error) {
	return p.buildGatewaySnapshot(bundle, gatewayName, p.Translator)
}

// BuildGatewaySnapshotForTarget 按 target 名称构建指定 Gateway 的运行时快照
func (p Pipeline) BuildGatewaySnapshotForTarget(bundle resource.Bundle, gatewayName, targetName string) (runtime.RuntimeSnapshot, error) {
	translator, ok := p.Registry.Get(targetName)
	if !ok {
		return runtime.RuntimeSnapshot{}, fmt.Errorf("target %q not registered", targetName)
	}

	return p.buildGatewaySnapshot(bundle, gatewayName, translator)
}

// BuildGatewaySnapshotsForTarget 按 target 名称构建所有 Gateway 的运行时快照
func (p Pipeline) BuildGatewaySnapshotsForTarget(bundle resource.Bundle, targetName string) ([]runtime.RuntimeSnapshot, error) {
	translator, ok := p.Registry.Get(targetName)
	if !ok {
		return nil, fmt.Errorf("target %q not registered", targetName)
	}

	snapshots := make([]runtime.RuntimeSnapshot, 0, len(bundle.Gateways))
	for _, gateway := range bundle.Gateways {
		snapshot, err := p.buildGatewaySnapshot(bundle, gateway.Metadata.Name, translator)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func (p Pipeline) buildGatewaySnapshot(bundle resource.Bundle, gatewayName string, translator target.Translator) (runtime.RuntimeSnapshot, error) {
	logical, err := p.Compiler.CompileGateway(bundle, gatewayName)
	if err != nil {
		return runtime.RuntimeSnapshot{}, fmt.Errorf("compile gateway %q: %w", gatewayName, err)
	}

	snapshot, err := translator.Translate(logical)
	if err != nil {
		return runtime.RuntimeSnapshot{}, fmt.Errorf("translate gateway %q to target %q: %w", gatewayName, translator.Target(), err)
	}

	return snapshot, nil
}
