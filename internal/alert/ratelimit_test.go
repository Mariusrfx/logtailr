package alert

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter()

	// First call should always pass
	if !rl.Allow("rule1", 100*time.Millisecond) {
		t.Error("first call should be allowed")
	}

	// Immediate second call should be blocked
	if rl.Allow("rule1", 100*time.Millisecond) {
		t.Error("second call within cooldown should be blocked")
	}

	// Different rule should pass
	if !rl.Allow("rule2", 100*time.Millisecond) {
		t.Error("different rule should be allowed")
	}

	// Wait for cooldown
	time.Sleep(150 * time.Millisecond)
	if !rl.Allow("rule1", 100*time.Millisecond) {
		t.Error("call after cooldown should be allowed")
	}
}

func TestRateLimiter_ZeroCooldown(t *testing.T) {
	rl := newRateLimiter()

	// Zero cooldown should always allow
	for i := 0; i < 10; i++ {
		if !rl.Allow("rule", 0) {
			t.Errorf("zero cooldown should always allow (iteration %d)", i)
		}
	}
}
