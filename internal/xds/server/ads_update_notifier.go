package server

import "sync"

// adsUpdateNotifier 把 RuntimeSnapshot 变化广播给当前已连接的 ADS stream
type adsUpdateNotifier struct {
	subscribers map[chan struct{}]struct{}
	mu          sync.Mutex
}

func newADSUpdateNotifier() *adsUpdateNotifier {
	return &adsUpdateNotifier{subscribers: map[chan struct{}]struct{}{}}
}

func (n *adsUpdateNotifier) Subscribe() (<-chan struct{}, func()) {
	updates := make(chan struct{}, 1)

	n.mu.Lock()
	n.subscribers[updates] = struct{}{}
	n.mu.Unlock()

	return updates, func() {
		n.mu.Lock()
		delete(n.subscribers, updates)
		close(updates)
		n.mu.Unlock()
	}
}

func (n *adsUpdateNotifier) Notify() {
	n.mu.Lock()
	defer n.mu.Unlock()

	for updates := range n.subscribers {
		select {
		case updates <- struct{}{}:
		default:
		}
	}
}
