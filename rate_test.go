package rate

import (
	"testing"
	"time"

	redis "gopkg.in/redis.v4"

	timerate "golang.org/x/time/rate"
)

func rateLimiter() *Limiter {
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{"server0": ":6379"},
	})
	if err := ring.FlushDb().Err(); err != nil {
		panic(err)
	}
	fallbackLimiter := timerate.NewLimiter(timerate.Every(time.Millisecond), 100)
	return NewLimiter(ring, fallbackLimiter)
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

func TestAllowRateMinute(t *testing.T) {
	const n = 2
	const dur = time.Minute

	l := rateLimiter()

	_, allow := l.AllowRate("rate", n*timerate.Every(dur))
	if !allow {
		t.Fatal("rate limited")
	}

	delay, allow := l.AllowRate("rate", n*timerate.Every(dur))
	if allow {
		t.Fatal("not rate limited")
	}
	if !durEqual(delay, dur/n) {
		t.Fatalf("got %s, wanted 0 < dur < %s", delay, dur/n)
	}
}

func TestAllowRateSecond(t *testing.T) {
	const n = 10
	const dur = time.Second

	l := rateLimiter()

	for i := 0; i < n; i++ {
		_, allow := l.AllowRate("rate", n*timerate.Every(dur))
		if !allow {
			t.Fatal("rate limited")
		}
	}

	delay, allow := l.AllowRate("rate", n*timerate.Every(dur))
	if allow {
		t.Fatal("not rate limited")
	}
	if !durEqual(delay, time.Second) {
		t.Fatalf("got %s, wanted 0 < dur < %s", delay, dur)
	}
}

func TestRedisIsDown(t *testing.T) {
	ring := redis.NewRing(&redis.RingOptions{})
	limiter := timerate.NewLimiter(timerate.Every(time.Second), 1)
	l := NewLimiter(ring, limiter)

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

func TestVerify(t *testing.T) {
	l := rateLimiter()

	rate, reset, allow := l.Verify("test_verify_id", 1, time.Minute)
	if !allow {
		t.Fatalf("rate limited with rate %d", rate)
	}
	if rate != 0 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur := time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Minute {
		t.Fatalf("got %s, wanted <= %s", dur, time.Minute)
	}

	l.Allow("test_verify_id", 1, time.Minute)
	l.Allow("test_verify_id", 1, time.Minute)

	rate, reset, allow = l.Verify("test_verify_id", 1, time.Minute)
	if allow {
		t.Fatalf("should rate limit with rate %d", rate)
	}
	if rate != 2 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur = time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Minute {
		t.Fatalf("got %s, wanted <= %s", dur, time.Minute)
	}
}

func TestVerifyMinute(t *testing.T) {
	l := rateLimiter()

	rate, reset, allow := l.VerifyMinute("test_verify_minute_id", 1)
	if !allow {
		t.Fatalf("rate limited with rate %d", rate)
	}
	if rate != 0 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur := time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Minute {
		t.Fatalf("got %s, wanted <= %s", dur, time.Minute)
	}

	l.AllowMinute("test_verify_minute_id", 1)
	l.AllowMinute("test_verify_minute_id", 1)

	rate, reset, allow = l.VerifyMinute("test_verify_minute_id", 1)
	if allow {
		t.Fatalf("should rate limit with rate %d", rate)
	}
	if rate != 2 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur = time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Minute {
		t.Fatalf("got %s, wanted <= %s", dur, time.Minute)
	}
}

func TestVerifyHour(t *testing.T) {
	l := rateLimiter()

	rate, reset, allow := l.VerifyHour("test_verify_hour_id", 1)
	if !allow {
		t.Fatalf("rate limited with rate %d", rate)
	}
	if rate != 0 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur := time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Hour {
		t.Fatalf("got %s, wanted <= %s", dur, time.Hour)
	}

	l.AllowHour("test_verify_hour_id", 1)
	l.AllowHour("test_verify_hour_id", 1)

	rate, reset, allow = l.VerifyHour("test_verify_hour_id", 1)
	if allow {
		t.Fatalf("should rate limit with rate %d", rate)
	}
	if rate != 2 {
		t.Fatalf("got %d, wanted 1", rate)
	}
	dur = time.Duration(reset-time.Now().Unix()) * time.Second
	if dur > time.Hour {
		t.Fatalf("got %s, wanted <= %s", dur, time.Hour)
	}
}
func durEqual(got, wanted time.Duration) bool {
	return got > 0 && got < wanted
}
