package proxy

import (
	"sync"
)

// Bulkhead limits concurrent downstream requests per provider.
type Bulkhead struct {
	limit int
	mu    sync.Mutex
	active map[string]int
}

func NewBulkhead(limit int) *Bulkhead {
	if limit <= 0 {
		return nil
	}
	return &Bulkhead{limit: limit, active: make(map[string]int)}
}

func (b *Bulkhead) TryAcquire(provider string) bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.active[provider] >= b.limit {
		return false
	}
	b.active[provider]++
	return true
}

func (b *Bulkhead) Release(provider string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.active[provider] > 0 {
		b.active[provider]--
	}
}
