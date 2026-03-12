package alert

import (
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter()

	if !rl.Allow("rule1", 100*time.Millisecond) {
		t.Error("first call should be allowed")
	}

	if rl.Allow("rule1", 100*time.Millisecond) {
		t.Error("second call within cooldown should be blocked")
	}

	if !rl.Allow("rule2", 100*time.Millisecond) {
		t.Error("different rule should be allowed")
	}

	time.Sleep(150 * time.Millisecond)
	if !rl.Allow("rule1", 100*time.Millisecond) {
		t.Error("call after cooldown should be allowed")
	}
}

func TestRateLimiter_ZeroCooldown(t *testing.T) {
	rl := newRateLimiter()

	for i := 0; i < 10; i++ {
		if !rl.Allow("rule", 0) {
			t.Errorf("zero cooldown should always allow (iteration %d)", i)
		}
	}
}
