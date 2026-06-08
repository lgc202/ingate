package ratelimit

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	proxytypes "github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"
)

const (
	sharedDataCounterKeyPrefix = "ingate:managed-rate-limit:local:"
	sharedDataCASMaxRetries    = 8
)

type counterStore interface {
	Increment(key string, now time.Time, windowSeconds int) (window, error)
}

type memoryCounterStore struct {
	windows map[string]window
}

func newMemoryCounterStore() *memoryCounterStore {
	return &memoryCounterStore{
		windows: make(map[string]window),
	}
}

func (s *memoryCounterStore) Increment(key string, now time.Time, windowSeconds int) (window, error) {
	current := s.windows[key]
	if current.start.IsZero() || now.Sub(current.start) >= time.Duration(windowSeconds)*time.Second {
		current = window{start: now}
	}
	current.count++
	s.windows[key] = current
	return current, nil
}

type sharedDataCounterStore struct{}

type sharedDataWindow struct {
	StartUnix int64 `json:"startUnix"`
	Count     int   `json:"count"`
}

func (s sharedDataCounterStore) Increment(key string, now time.Time, windowSeconds int) (window, error) {
	sharedKey := sharedDataCounterKeyPrefix + key
	for range sharedDataCASMaxRetries {
		current, cas, err := s.get(sharedKey)
		if err != nil {
			return window{}, err
		}
		if current.start.IsZero() || now.Sub(current.start) >= time.Duration(windowSeconds)*time.Second {
			current = window{start: now}
		}
		current.count++
		if err := s.set(sharedKey, current, cas); err != nil {
			if errors.Is(err, proxytypes.ErrorStatusCasMismatch) {
				continue
			}
			return window{}, err
		}
		return current, nil
	}
	return window{}, fmt.Errorf("shared data CAS retry exhausted for %q", key)
}

func (s sharedDataCounterStore) get(key string) (window, uint32, error) {
	data, cas, err := proxywasm.GetSharedData(key)
	if err != nil {
		if errors.Is(err, proxytypes.ErrorStatusNotFound) {
			return window{}, 0, nil
		}
		return window{}, 0, err
	}
	if len(data) == 0 {
		return window{}, cas, nil
	}

	var value sharedDataWindow
	if err := json.Unmarshal(data, &value); err != nil {
		return window{}, 0, err
	}
	return window{
		start: time.Unix(value.StartUnix, 0),
		count: value.Count,
	}, cas, nil
}

func (s sharedDataCounterStore) set(key string, current window, cas uint32) error {
	data, err := json.Marshal(sharedDataWindow{
		StartUnix: current.start.Unix(),
		Count:     current.count,
	})
	if err != nil {
		return err
	}
	return proxywasm.SetSharedData(key, data, cas)
}
