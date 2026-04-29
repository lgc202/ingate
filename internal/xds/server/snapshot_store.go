package server

import (
	"sync"

	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
)

// snapshotStore 保存指定 target 的 RuntimeSnapshot，供后续 xDS 协议层读取
type snapshotStore struct {
	target    string
	snapshots map[string]*resource.RuntimeSnapshot
	mu        sync.RWMutex
}

func newSnapshotStore(target string) *snapshotStore {
	return &snapshotStore{
		target:    target,
		snapshots: map[string]*resource.RuntimeSnapshot{},
	}
}

func (s *snapshotStore) Apply(snapshot *resource.RuntimeSnapshot) bool {
	if snapshot.Spec.Target != s.target {
		return false
	}

	s.mu.Lock()
	s.snapshots[snapshot.Spec.Gateway] = snapshot.DeepCopy()
	s.mu.Unlock()
	return true
}

func (s *snapshotStore) Delete(snapshot *resource.RuntimeSnapshot) bool {
	if snapshot.Spec.Target != s.target {
		return false
	}

	s.mu.Lock()
	delete(s.snapshots, snapshot.Spec.Gateway)
	s.mu.Unlock()
	return true
}

func (s *snapshotStore) Get(gateway string) (*resource.RuntimeSnapshot, bool) {
	s.mu.RLock()
	snapshot, ok := s.snapshots[gateway]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return snapshot.DeepCopy(), true
}

func (s *snapshotStore) List() []*resource.RuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshots := make([]*resource.RuntimeSnapshot, 0, len(s.snapshots))
	for _, snapshot := range s.snapshots {
		snapshots = append(snapshots, snapshot.DeepCopy())
	}
	return snapshots
}

func (s *snapshotStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.snapshots)
}
