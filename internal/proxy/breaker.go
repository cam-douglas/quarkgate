package proxy

import (
	"sync"
	"time"
)

// CircuitBreaker opens when failure rate exceeds threshold within window.
type CircuitBreaker struct {
	mu           sync.Mutex
	failures     int
	successes    int
	openUntil    time.Time
	threshold    float64
	cooldown     time.Duration
}

func NewCircuitBreaker(threshold float64, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{threshold: threshold, cooldown: cooldown}
}

func (cb *CircuitBreaker) Allow(provider string) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if time.Now().Before(cb.openUntil) {
		return false
	}
	total := cb.failures + cb.successes
	if total < 5 {
		return true
	}
	rate := float64(cb.failures) / float64(total)
	if rate > cb.threshold {
		cb.openUntil = time.Now().Add(cb.cooldown)
		cb.failures = 0
		cb.successes = 0
		return false
	}
	return true
}

func (cb *CircuitBreaker) Record(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if success {
		cb.successes++
	} else {
		cb.failures++
	}
}
