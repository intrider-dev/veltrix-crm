package httpx

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

type RateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]bucket
	capacity    float64
	refill      float64
	maxEntries  int
	lastCleanup time.Time
}

func NewRateLimiter(capacity int, refillPerSecond float64, maxEntries int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]bucket), capacity: float64(capacity), refill: refillPerSecond,
		maxEntries: maxEntries, lastCleanup: time.Now(),
	}
}

func (limiter *RateLimiter) Allow(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if now.Sub(limiter.lastCleanup) > 5*time.Minute || len(limiter.buckets) >= limiter.maxEntries {
		cutoff := now.Add(-15 * time.Minute)
		for candidate, current := range limiter.buckets {
			if current.last.Before(cutoff) {
				delete(limiter.buckets, candidate)
			}
		}
		limiter.lastCleanup = now
	}
	current, ok := limiter.buckets[key]
	if !ok {
		if len(limiter.buckets) >= limiter.maxEntries {
			return false
		}
		current = bucket{tokens: limiter.capacity, last: now}
	}
	elapsed := now.Sub(current.last).Seconds()
	current.tokens = min(limiter.capacity, current.tokens+elapsed*limiter.refill)
	current.last = now
	if current.tokens < 1 {
		limiter.buckets[key] = current
		return false
	}
	current.tokens--
	limiter.buckets[key] = current
	return true
}
