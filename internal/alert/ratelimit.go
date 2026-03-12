package alert

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	lastFire map[string]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		lastFire: make(map[string]time.Time),
	}
}

func (rl *rateLimiter) Allow(ruleName string, cooldown time.Duration) bool {
	if cooldown <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	last, ok := rl.lastFire[ruleName]
	if ok && time.Since(last) < cooldown {
		return false
	}

	rl.lastFire[ruleName] = time.Now()
	return true
}
