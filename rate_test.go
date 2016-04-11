package rate_test

import (
	"testing"
	"time"

	timerate "golang.org/x/time/rate"
	"gopkg.in/redis.v4"

	"gopkg.in/go-redis/rate.v4"
)

func rateLimiter() *rate.Limiter {
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{"server0": ":6379"},
	})
	if err := ring.FlushDb().Err(); err != nil {
		panic(err)
	}
	fallbackLimiter := timerate.NewLimiter(timerate.Every(time.Millisecond), 100)
	return rate.NewLimiter(ring, fallbackLimiter)
}

func TestAllow(t *testing.T) {
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

func TestAllowRate(t *testing.T) {
	l := rateLimiter()

	_, allow := l.AllowRate("rate", 2*timerate.Every(time.Minute))
	if !allow {
		t.Fatal("rate limited")
	}

	delay, allow := l.AllowRate("rate", 2*timerate.Every(time.Minute))
	if allow {
		t.Fatal("not rate limited")
	}
	if !(delay > 0 && delay < 30*time.Second) {
		t.Fatalf("got %s, wanted 0 < dur < 30s", delay)
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
