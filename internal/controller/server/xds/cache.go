package xds

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
)

// CacheKey 是一套 Ingate 配置域在 Snapshot Cache 中使用的固定键
const CacheKey = "ingate"

type nodeHash struct{}

var _ cachev3.NodeHash = nodeHash{}

// NewSnapshotCache 创建 ADS 与 Delivery 共享的 Snapshot Cache
func NewSnapshotCache(logger log.Logger) cachev3.SnapshotCache {
	return cachev3.NewSnapshotCache(true, nodeHash{}, logger)
}

// ID 将当前 Controller 管理的所有 Envoy 实例映射到同一配置域，Node ID 只用于连接唯一性和 ACK/NACK 观测
func (nodeHash) ID(*corev3.Node) string {
	return CacheKey
}
