package redis_rate

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v7"
	"golang.org/x/time/rate"
)

const redisPrefix = "rate"

type rediser interface {
	Del(...string) *redis.IntCmd
	Pipelined(func(pipe redis.Pipeliner) error) ([]redis.Cmder, error)
}

// Limiter controls how frequently events are allowed to happen.
type Limiter struct {
	redis rediser

	// Optional fallback limiter used when Redis is unavailable.
	Fallback *rate.Limiter
}

func NewLimiter(redis rediser) *Limiter {
	return &Limiter{
		redis: redis,
	}
}

// Reset resets the rate limit for the name in the given rate limit period.
func (l *Limiter) Reset(name string, period time.Duration) error {
	secs := int64(period / time.Second)
	slot := time.Now().Unix() / secs

	name = allowName(name, slot)
	return l.redis.Del(name).Err()
}

// ResetRate resets the rate limit for the name and limit.
func (l *Limiter) ResetRate(name string, rateLimit rate.Limit) error {
	if rateLimit == 0 {
		return nil
	}
	if rateLimit == rate.Inf {
		return nil
	}

	_, period := limitPeriod(rateLimit)
	slot := time.Now().UnixNano() / period.Nanoseconds()

	name = allowRateName(name, period, slot)
	return l.redis.Del(name).Err()
}

// AllowN reports whether an event with given name may happen at time now.
// It allows up to maxn events within period, with each interaction
// incrementing the limit by n.
func (l *Limiter) AllowN(
	name string, maxn int64, period time.Duration, n int64,
) (count int64, delay time.Duration, allow bool) {
	secs := int64(period / time.Second)
	utime := time.Now().Unix()
	slot := utime / secs
	delay = time.Duration((slot+1)*secs-utime) * time.Second

	if l.Fallback != nil {
		allow = l.Fallback.Allow()
	}

	name = allowName(name, slot)
	count, err := l.incr(name, period, n)
	if err == nil {
		allow = count <= maxn
	}

	return count, delay, allow
}

// Allow is shorthand for AllowN(name, max, period, 1).
func (l *Limiter) Allow(name string, maxn int64, period time.Duration) (count int64, delay time.Duration, allow bool) {
	return l.AllowN(name, maxn, period, 1)
}

// AllowMinute is shorthand for Allow(name, maxn, time.Minute).
func (l *Limiter) AllowMinute(name string, maxn int64) (count int64, delay time.Duration, allow bool) {
	return l.Allow(name, maxn, time.Minute)
}

// AllowHour is shorthand for Allow(name, maxn, time.Hour).
func (l *Limiter) AllowHour(name string, maxn int64) (count int64, delay time.Duration, allow bool) {
	return l.Allow(name, maxn, time.Hour)
}

// AllowRate reports whether an event may happen at time now.
// It allows up to rateLimit events each second.
func (l *Limiter) AllowRate(name string, rateLimit rate.Limit) (delay time.Duration, allow bool) {
	if rateLimit == 0 {
		return 0, false
	}
	if rateLimit == rate.Inf {
		return 0, true
	}

	limit, period := limitPeriod(rateLimit)
	now := time.Now()
	slot := now.UnixNano() / period.Nanoseconds()

	name = allowRateName(name, period, slot)
	count, err := l.incr(name, period, 1)
	if err == nil {
		allow = count <= limit
	} else if l.Fallback != nil {
		allow = l.Fallback.Allow()
	}

	if !allow {
		delay = time.Duration(slot+1)*period - time.Duration(now.UnixNano())
	}

	return delay, allow
}

func limitPeriod(rl rate.Limit) (limit int64, period time.Duration) {
	period = time.Second
	if rl < 1 {
		limit = 1
		period *= time.Duration(1 / rl)
	} else {
		limit = int64(rl)
	}
	return limit, period
}

func (l *Limiter) incr(name string, period time.Duration, n int64) (int64, error) {
	var incr *redis.IntCmd
	_, err := l.redis.Pipelined(func(pipe redis.Pipeliner) error {
		incr = pipe.IncrBy(name, n)
		pipe.Expire(name, period+30*time.Second)
		return nil
	})

	rate, _ := incr.Result()
	return rate, err
}

func allowName(name string, slot int64) string {
	return fmt.Sprintf("%s:%s-%d", redisPrefix, name, slot)
}

func allowRateName(name string, period time.Duration, slot int64) string {
	return fmt.Sprintf("%s:%s-%d-%d", redisPrefix, name, period, slot)
}
