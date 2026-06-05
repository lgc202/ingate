package runtimegroup

import resource "github.com/lgc202/ingate/pkg/apis/gateway/v1"

const (
	// DefaultID 是第一阶段内置的数据面运行组标识
	DefaultID = "default"
	// DefaultTargetID 是第一阶段内置运行组对应的 target 标识
	DefaultTargetID = "xds"

	defaultDisplayName = "默认运行组"
	defaultDescription = "系统内置运行组，默认投放到 xDS target"
)

// ListResult 是 RuntimeGroup 列表用例结果
type ListResult struct {
	RuntimeGroups []resource.RuntimeGroup
}
