package redisabi

import (
	"fmt"
	"sync"
)

type callKey struct {
	pluginContextID uint32
	calloutID       uint32
}

type callbackRegistry struct {
	mu        sync.Mutex
	callbacks map[callKey]Callback
}

func newCallbackRegistry() *callbackRegistry {
	return &callbackRegistry{callbacks: make(map[callKey]Callback)}
}

func (r *callbackRegistry) add(key callKey, callback Callback) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.callbacks[key]; exists {
		return fmt.Errorf("redis ABI callout %d already exists for plugin context %d", key.calloutID, key.pluginContextID)
	}
	r.callbacks[key] = callback
	return nil
}

func (r *callbackRegistry) take(key callKey) (Callback, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	callback, exists := r.callbacks[key]
	if exists {
		delete(r.callbacks, key)
	}
	return callback, exists
}
