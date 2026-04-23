package cache

import (
	"sort"
	"sync"
	"time"

	"github.com/lgc202/ingate/internal/controlplane/controller/shared"
	"github.com/lgc202/ingate/internal/controlplane/xds/translate"
	configsyncv1 "github.com/lgc202/ingate/pkg/generated/proto/ingate/configsync/v1"
)

type Snapshot struct {
	Key             shared.ObjectKey
	SourceVersion   string
	PublishVersion  string
	Runtime         *translate.RuntimeConfig
	EffectiveConfig *configsyncv1.EffectiveConfig
	UpdatedAt       time.Time
}

type Cache struct {
	mu        sync.RWMutex
	snapshots map[string]Snapshot
}

func New() *Cache {
	return &Cache{snapshots: make(map[string]Snapshot)}
}

func (c *Cache) Upsert(snapshot Snapshot) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now()
	}
	c.snapshots[snapshot.Key.String()] = snapshot
}

func (c *Cache) Delete(key shared.ObjectKey) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.snapshots, key.String())
}

func (c *Cache) Get(key shared.ObjectKey) (Snapshot, bool) {
	if c == nil {
		return Snapshot{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	snapshot, ok := c.snapshots[key.String()]
	return snapshot, ok
}

func (c *Cache) List() []Snapshot {
	if c == nil {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Snapshot, 0, len(c.snapshots))
	for _, snapshot := range c.snapshots {
		out = append(out, snapshot)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key.String() < out[j].Key.String()
	})
	return out
}

func (c *Cache) ResolveBackend(name, backendType string) (*translate.RuntimeBackend, bool) {
	if c == nil || name == "" {
		return nil, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, snapshot := range c.snapshots {
		if snapshot.Runtime == nil {
			continue
		}
		for i := range snapshot.Runtime.Backends {
			backend := &snapshot.Runtime.Backends[i]
			if backend.Name != name {
				continue
			}
			if backendType != "" && backend.Type != "" && backend.Type != backendType {
				continue
			}
			return backend, true
		}
	}
	return nil, false
}
