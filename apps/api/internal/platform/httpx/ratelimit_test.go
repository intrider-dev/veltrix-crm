package httpx

import (
	"testing"
	"time"
)

func TestRateLimiterRefills(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(2, 1, 10)
	now := time.Unix(0, 0)
	if !limiter.Allow("client", now) || !limiter.Allow("client", now) {
		t.Fatal("initial capacity rejected")
	}
	if limiter.Allow("client", now) {
		t.Fatal("request above capacity accepted")
	}
	if !limiter.Allow("client", now.Add(time.Second)) {
		t.Fatal("refilled request rejected")
	}
}

func TestRateLimiterFailsClosedAtKeyCapacity(t *testing.T) {
	t.Parallel()
	limiter := NewRateLimiter(1, 1, 2)
	now := time.Unix(0, 0)
	if !limiter.Allow("first", now) || !limiter.Allow("second", now) {
		t.Fatal("requests within key capacity were rejected")
	}
	if limiter.Allow("third", now) {
		t.Fatal("new key above bounded capacity was accepted")
	}
	if limiter.Allow("first", now) {
		t.Fatal("existing exhausted bucket bypassed its token limit")
	}
	if !limiter.Allow("first", now.Add(time.Second)) {
		t.Fatal("existing bucket did not continue to refill at key capacity")
	}
}
