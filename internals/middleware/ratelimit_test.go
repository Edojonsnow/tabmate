package middleware

import (
	"testing"
	"time"
)

func TestRateLimiterAllowsBurstThenRejects(t *testing.T) {
	limiter := &rateLimiter{
		buckets: make(map[string]*rateLimitBucket),
		config: RateLimitConfig{
			Name:   "test",
			Limit:  2,
			Window: time.Minute,
			Burst:  2,
		},
	}

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)

	if allowed, remaining, retryAfter := limiter.allow("key", now); !allowed || remaining != 1 || retryAfter != 0 {
		t.Fatalf("first request = allowed %v, remaining %d, retryAfter %s", allowed, remaining, retryAfter)
	}
	if allowed, remaining, retryAfter := limiter.allow("key", now); !allowed || remaining != 0 || retryAfter != 0 {
		t.Fatalf("second request = allowed %v, remaining %d, retryAfter %s", allowed, remaining, retryAfter)
	}
	if allowed, remaining, retryAfter := limiter.allow("key", now); allowed || remaining != 0 || retryAfter <= 0 {
		t.Fatalf("third request = allowed %v, remaining %d, retryAfter %s", allowed, remaining, retryAfter)
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	limiter := &rateLimiter{
		buckets: make(map[string]*rateLimitBucket),
		config: RateLimitConfig{
			Name:   "test",
			Limit:  2,
			Window: time.Minute,
			Burst:  2,
		},
	}

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	limiter.allow("key", now)
	limiter.allow("key", now)

	allowed, remaining, retryAfter := limiter.allow("key", now.Add(30*time.Second))
	if !allowed || remaining != 0 || retryAfter != 0 {
		t.Fatalf("refilled request = allowed %v, remaining %d, retryAfter %s", allowed, remaining, retryAfter)
	}
}
