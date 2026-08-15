package xds

import (
	"context"
	"fmt"

	cachetypes "github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	resourcev3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/lgc202/ingate/internal/controller/biz/compiler"
)

// Publisher 将编译结果转换为一致的 Snapshot 并写入 xDS Cache
type Publisher struct {
	cache cachev3.SnapshotCache
}

// NewPublisher 创建当前单配置域的 xDS 发布器
func NewPublisher(cache cachev3.SnapshotCache) *Publisher {
	return &Publisher{cache: cache}
}

// Publish 构造完整 Snapshot，并原子替换当前配置域的可见版本
func (p *Publisher) Publish(ctx context.Context, version string, config compiler.EnvoyConfig) error {
	snapshot, err := newSnapshot(version, config)
	if err != nil {
		return fmt.Errorf("build xDS snapshot %q: %w", version, err)
	}
	if err := p.cache.SetSnapshot(ctx, CacheKey, snapshot); err != nil {
		return fmt.Errorf("set xDS snapshot %q: %w", version, err)
	}
	return nil
}

func newSnapshot(version string, config compiler.EnvoyConfig) (*cachev3.Snapshot, error) {
	resources := map[resourcev3.Type][]cachetypes.Resource{
		resourcev3.ListenerType: make([]cachetypes.Resource, 0, len(config.Listeners)),
		resourcev3.RouteType:    make([]cachetypes.Resource, 0, len(config.Routes)),
		resourcev3.ClusterType:  make([]cachetypes.Resource, 0, len(config.Clusters)),
		resourcev3.EndpointType: make([]cachetypes.Resource, 0, len(config.Endpoints)),
	}
	for _, listener := range config.Listeners {
		resources[resourcev3.ListenerType] = append(resources[resourcev3.ListenerType], listener)
	}
	for _, route := range config.Routes {
		resources[resourcev3.RouteType] = append(resources[resourcev3.RouteType], route)
	}
	for _, cluster := range config.Clusters {
		resources[resourcev3.ClusterType] = append(resources[resourcev3.ClusterType], cluster)
	}
	for _, endpoint := range config.Endpoints {
		resources[resourcev3.EndpointType] = append(resources[resourcev3.EndpointType], endpoint)
	}

	snapshot, err := cachev3.NewSnapshot(version, resources)
	if err != nil {
		return nil, fmt.Errorf("create xDS snapshot: %w", err)
	}
	if err := snapshot.Consistent(); err != nil {
		return nil, fmt.Errorf("check xDS snapshot consistency: %w", err)
	}
	return snapshot, nil
}

// HasVersion 确认四类动态资源已经作为同一版本进入 Snapshot Cache
//
// SetSnapshot 可能在调用方 context 取消后返回错误，但 Cache 已经完成替换；
// Delivery 需要据此决定是否仍应跟踪该 Candidate 的 ACK/NACK
func (p *Publisher) HasVersion(version string) bool {
	snapshot, err := p.cache.GetSnapshot(CacheKey)
	if err != nil {
		return false
	}
	for _, typeURL := range []string{
		resourcev3.ListenerType,
		resourcev3.RouteType,
		resourcev3.ClusterType,
		resourcev3.EndpointType,
	} {
		if snapshot.GetVersion(typeURL) != version {
			return false
		}
	}
	return true
}
