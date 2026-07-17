package redisabi

import (
	"fmt"
	"sync"
)

type callKey struct {
	pluginContextID uint32
	calloutID       uint32
}

type contextKey struct {
	pluginContextID uint32
	httpContextID   uint32
}

type callbackRecord struct {
	pluginContextID uint32
	httpContextID   uint32
	callback        Callback
}

type callbackRegistry struct {
	mu        sync.Mutex
	callbacks map[callKey]callbackRecord
	contexts  map[contextKey]struct{}
}

func newCallbackRegistry() *callbackRegistry {
	return &callbackRegistry{
		callbacks: make(map[callKey]callbackRecord),
		contexts:  make(map[contextKey]struct{}),
	}
}

func (r *callbackRegistry) registerContext(key contextKey) {
	r.mu.Lock()
	r.contexts[key] = struct{}{}
	r.mu.Unlock()
}

func (r *callbackRegistry) closeContext(key contextKey) {
	r.mu.Lock()
	delete(r.contexts, key)
	r.mu.Unlock()
}

func (r *callbackRegistry) contextAlive(key contextKey) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, exists := r.contexts[key]
	return exists
}

func (r *callbackRegistry) add(key callKey, record callbackRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.callbacks[key]; exists {
		return fmt.Errorf("redis ABI callout %d already exists for plugin context %d", key.calloutID, key.pluginContextID)
	}
	r.callbacks[key] = record
	return nil
}

func (r *callbackRegistry) get(key callKey) (callbackRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, exists := r.callbacks[key]
	return record, exists
}

func (r *callbackRegistry) remove(key callKey) {
	r.mu.Lock()
	delete(r.callbacks, key)
	r.mu.Unlock()
}
