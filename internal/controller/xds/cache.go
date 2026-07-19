package xds

import (
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/log"
)

// NewSnapshotCache 创建 ADS 与 Delivery 共享的 Snapshot Cache
func NewSnapshotCache(logger log.Logger) cachev3.SnapshotCache {
	return cachev3.NewSnapshotCache(true, NodeHash{}, logger)
}
