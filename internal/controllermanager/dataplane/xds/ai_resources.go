package xds

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"google.golang.org/protobuf/proto"

	aidataplanev1 "github.com/lgc202/ingate/api/aidataplane/v1"
)

// aiResourceCache 只拥有已编译资源，不解析业务引用或执行路由裁决。
// LinearCache 延迟编码和计算内容版本，因此安装后持有的 protobuf 必须保持只读。
type aiResourceCache struct {
	cache       *cachev3.LinearCache
	initialized atomic.Bool
	// mu 串行保护资源比较、所有权移交和失败通知的重试集合。
	mu        sync.Mutex
	resources map[string]*aidataplanev1.Resource
	pending   map[string]bool
}

func newAIResourceCache() *aiResourceCache {
	return &aiResourceCache{
		cache:     cachev3.NewLinearCache(aidataplanev1.ResourceType),
		resources: make(map[string]*aidataplanev1.Resource),
		pending:   make(map[string]bool),
	}
}

func (c *aiResourceCache) publish(resources []*aidataplanev1.Resource) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	desired := make(map[string]*aidataplanev1.Resource, len(resources))
	for _, resource := range resources {
		if _, exists := desired[resource.Name]; exists {
			return fmt.Errorf("duplicate AI Proxy resource %q", resource.Name)
		}

		if previous := c.resources[resource.Name]; proto.Equal(previous, resource) {
			desired[resource.Name] = previous
			continue
		}

		// 在安装前完成可编码性检查，避免延迟编码失败污染最近有效缓存。
		owned := proto.CloneOf(resource)
		if _, err := cachev3.MarshalResource(owned); err != nil {
			return fmt.Errorf("encode AI Proxy resource %q: %w", resource.Name, err)
		}

		desired[resource.Name] = owned
	}

	for name, resource := range desired {
		if resource != c.resources[name] {
			c.pending[name] = true
		}
	}

	for name := range c.resources {
		if _, exists := desired[name]; !exists {
			c.pending[name] = true
		}
	}

	if c.initialized.Load() && len(c.pending) == 0 {
		return nil
	}

	updates := make(map[string]types.Resource)
	var removals []string
	for name := range c.pending {
		if resource, exists := desired[name]; exists {
			updates[name] = resource
		} else {
			removals = append(removals, name)
		}
	}

	// 原生 UpdateResources 先安装整批再通知；失败也不能回滚已安装的资源。
	// 保留受影响名称，下一次对最新期望重试，包括通知失败后又被修改或撤销的资源。
	c.resources = desired
	if err := c.cache.UpdateResources(updates, removals); err != nil {
		c.initialized.Store(true)

		return fmt.Errorf("notify AI Proxy resource watches: %w", err)
	}

	c.initialized.Store(true)
	clear(c.pending)

	return nil
}
