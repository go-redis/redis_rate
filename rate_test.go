package rate_test

import (
	"testing"
	"time"

	timerate "golang.org/x/time/rate"
	"gopkg.in/redis.v3"

	"github.com/go-redis/rate"
)

func rateLimiter() *rate.Limiter {
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{"0": ":6379"},
	})
	ring.FlushDb()
	limiter := timerate.NewLimiter(timerate.Every(time.Millisecond), 100)
	return rate.NewLimiter(ring, limiter)
}

func TestLimit(t *testing.T) {
	l := rateLimiter()

	rate, reset, allow := l.Allow("test_id", 1, time.Minute)
	if !allow {
		t.Fatalf("rate limited with rate %d", rate)
	}
	if rate != 1 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur := time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Minute {
		t.Fatalf("got %s, wanted <= %s", dur, time.Minute)
	}

	rate, _, allow = l.Allow("test_id", 1, time.Minute)
	if allow {
		t.Fatalf("not rate limited with rate %d", rate)
	}
	if rate != 2 {
		t.Fatalf("got %d, wanted 2", rate)
	}
}

func TestRedisIsDown(t *testing.T) {
	ring := redis.NewRing(&redis.RingOptions{})
	limiter := timerate.NewLimiter(timerate.Every(time.Second), 1)
	l := rate.NewLimiter(ring, limiter)

	rate, _, allow := l.AllowMinute("test_id", 1)
	if !allow {
		t.Fatalf("rate limited with rate %d", rate)
	}
	if rate != 0 {
		t.Fatalf("got %d, wanted 0", rate)
	}

	rate, _, allow = l.AllowMinute("test_id", 1)
	if allow {
		t.Fatalf("not rate limited with rate %d", rate)
	}
	if rate != 0 {
		t.Fatalf("got %d, wanted 0", rate)
	}
}
