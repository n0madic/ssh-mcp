package security

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter provides per-host rate limiting using token buckets.
type RateLimiter struct {
	mu           sync.RWMutex
	limiters     map[string]*rate.Limiter
	lastAccessed map[string]time.Time
	rpm          int // requests per minute
}

// NewRateLimiter creates a new per-host rate limiter.
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		limiters:     make(map[string]*rate.Limiter),
		lastAccessed: make(map[string]time.Time),
		rpm:          requestsPerMinute,
	}
}

// Allow checks if a request to the given host is allowed.
func (r *RateLimiter) Allow(host string) error {
	limiter := r.getLimiter(host)
	if !limiter.Allow() {
		return fmt.Errorf("rate limit exceeded for host %q (limit: %d requests/min)", host, r.rpm)
	}
	return nil
}

// Cleanup removes rate limiter entries that haven't been accessed for maxAge.
func (r *RateLimiter) Cleanup(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	removed := 0
	for host, lastUsed := range r.lastAccessed {
		if now.Sub(lastUsed) > maxAge {
			delete(r.limiters, host)
			delete(r.lastAccessed, host)
			removed++
		}
	}
	return removed
}

// StartCleanup starts a background goroutine that periodically evicts stale entries.
func (r *RateLimiter) StartCleanup(ctx context.Context, interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if removed := r.Cleanup(maxAge); removed > 0 {
					log.Printf("Rate limiter cleanup: removed %d stale entries", removed)
				}
			}
		}
	}()
}

func (r *RateLimiter) getLimiter(host string) *rate.Limiter {
	r.mu.RLock()
	limiter, exists := r.limiters[host]
	r.mu.RUnlock()

	if exists {
		r.mu.Lock()
		r.lastAccessed[host] = time.Now()
		r.mu.Unlock()
		return limiter
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if limiter, exists = r.limiters[host]; exists {
		r.lastAccessed[host] = time.Now()
		return limiter
	}

	// Token bucket: rate = rpm/60 tokens per second, burst = rpm/10 (at least 1).
	rps := rate.Limit(float64(r.rpm) / 60.0)
	burst := max(r.rpm/10, 1)

	limiter = rate.NewLimiter(rps, burst)
	r.limiters[host] = limiter
	r.lastAccessed[host] = time.Now()
	return limiter
}
