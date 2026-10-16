package aidataplane

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// TargetHeader 只在受保护的 Go → Envoy 出口上传递已选目标，不能从客户端透传。
const TargetHeader = "x-ingate-ai-target"

// modelTarget 固定单个出口的内容编码；它不是新增领域资源或整个配置的版本。
type modelTarget struct {
	Backend Backend        `json:"backend"`
	Network NetworkBackend `json:"network"`
}

// mcpTarget 固定单个 MCP 网络出口的内容编码；入口路径由 Go 应用，不参与出口选择。
type mcpTarget struct {
	Network NetworkBackend `json:"network"`
}

// ModelTargetName 根据已校验的协议绑定和网络快照生成不可复用到不同内容的出口名。
// 端点先复制排序，不修改输入；改变无关路由不会改变该目标。它不冻结 DNS 解析结果，
// 也不提供跨组件原子发布、即时撤销或凭据轮转保证。
func ModelTargetName(backend Backend, network NetworkBackend) (string, error) {
	if backend.NetworkBackendName != network.Name || network.Name == "" {
		return "", fmt.Errorf("model backend %q does not reference network backend %q", backend.Name, network.Name)
	}

	network.Endpoints = slices.Clone(network.Endpoints)
	slices.SortFunc(network.Endpoints, compareEndpoints)

	data, err := json.Marshal(modelTarget{Backend: backend, Network: network})
	if err != nil {
		return "", fmt.Errorf("encode model target: %w", err)
	}

	sum := sha256.Sum256(data)

	return "ai/target/" + hex.EncodeToString(sum[:]), nil
}

// MCPTargetName 根据已校验的网络快照生成 MCP 单后端出口名，与 ModelTargetName 采用同样的
// 内容寻址：端点先复制排序，不修改输入，只有网络内容改变才生成新名称；不同入口路径
// 复用同一网络时共享出口。它不冻结 DNS 解析结果，也不承诺即时撤销。
func MCPTargetName(network NetworkBackend) (string, error) {
	if network.Name == "" {
		return "", errors.New("MCP network backend name must be nonempty")
	}

	network.Endpoints = slices.Clone(network.Endpoints)
	slices.SortFunc(network.Endpoints, compareEndpoints)

	data, err := json.Marshal(mcpTarget{Network: network})
	if err != nil {
		return "", fmt.Errorf("encode MCP target: %w", err)
	}

	sum := sha256.Sum256(data)

	return "ai/mcp-target/" + hex.EncodeToString(sum[:]), nil
}

func compareEndpoints(a, b Endpoint) int {
	if order := cmp.Compare(a.Address, b.Address); order != 0 {
		return order
	}

	if order := cmp.Compare(a.Port, b.Port); order != 0 {
		return order
	}

	return cmp.Compare(a.Weight, b.Weight)
}
