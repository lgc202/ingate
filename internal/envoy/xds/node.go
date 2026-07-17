package xds

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

// CacheKey 是一套 Ingate 配置域在 Snapshot Cache 中使用的固定键
const CacheKey = "ingate"

// NodeHash 将所有 Envoy 实例映射到同一套 Ingate 配置
type NodeHash struct{}

var _ cachev3.NodeHash = NodeHash{}

// ID 返回固定 CacheKey，Node ID 仅用于连接唯一性和 ACK/NACK 观测
func (NodeHash) ID(*corev3.Node) string {
	return CacheKey
}
