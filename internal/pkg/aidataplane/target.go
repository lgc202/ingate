package aidataplane

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// ModelTargetName 根据已校验的协议绑定和网络快照生成不可复用到不同内容的出口名。
// 端点先复制排序，不修改输入；改变无关路由不会改变该目标。它不冻结 DNS 解析结果，
// 也不提供跨组件原子发布、即时撤销或凭据轮转保证。
func ModelTargetName(backend Backend, network NetworkBackend) (string, error) {
	if backend.NetworkBackendName != network.Name || network.Name == "" {
		return "", fmt.Errorf("model backend %q does not reference network backend %q", backend.Name, network.Name)
	}

	network.Endpoints = slices.Clone(network.Endpoints)
	slices.SortFunc(network.Endpoints, func(a, b Endpoint) int {
		if order := cmp.Compare(a.Address, b.Address); order != 0 {
			return order
		}
		if order := cmp.Compare(a.Port, b.Port); order != 0 {
			return order
		}

		return cmp.Compare(a.Weight, b.Weight)
	})

	data, err := json.Marshal(modelTarget{Backend: backend, Network: network})
	if err != nil {
		return "", fmt.Errorf("encode model target: %w", err)
	}

	sum := sha256.Sum256(data)
	return "ai/target/" + hex.EncodeToString(sum[:]), nil
}
