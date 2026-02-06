package security

import (
	"testing"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(60)

	// First request should be allowed.
	if err := rl.Allow("host1"); err != nil {
		t.Errorf("expected first request allowed: %v", err)
	}
}

func TestRateLimiter_PerHost(t *testing.T) {
	rl := NewRateLimiter(60)

	// Different hosts should have independent limiters.
	if err := rl.Allow("host1"); err != nil {
		t.Errorf("expected host1 allowed: %v", err)
	}
	if err := rl.Allow("host2"); err != nil {
		t.Errorf("expected host2 allowed: %v", err)
	}
}

func TestRateLimiter_BurstExceeded(t *testing.T) {
	// Very low rate limit to test burst.
	rl := NewRateLimiter(1)

	// First request should be allowed (burst of at least 1).
	if err := rl.Allow("host1"); err != nil {
		t.Errorf("expected first request allowed: %v", err)
	}

	// Subsequent requests should eventually be denied.
	denied := false
	for i := 0; i < 10; i++ {
		if err := rl.Allow("host1"); err != nil {
			denied = true
			break
		}
	}
	if !denied {
		t.Error("expected rate limit to eventually deny requests")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(60)

	// Create a limiter for host1.
	if err := rl.Allow("host1"); err != nil {
		t.Errorf("expected first request allowed: %v", err)
	}

	// Call Cleanup with maxAge=0 to evict everything.
	removed := rl.Cleanup(0)
	if removed != 1 {
		t.Errorf("expected 1 entry removed, got %d", removed)
	}

	// Verify next Allow("host1") still works (new limiter created).
	if err := rl.Allow("host1"); err != nil {
		t.Errorf("expected request allowed after cleanup: %v", err)
	}
}
