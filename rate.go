package rate // import "gopkg.in/go-redis/rate.v4"

import (
	"strconv"
	"time"

	timerate "golang.org/x/time/rate"

	"gopkg.in/redis.v4"
)

const redisPrefix = "rate:"

type rediser interface {
	Pipelined(func(pipe *redis.Pipeline) error) ([]redis.Cmder, error)
}

type Limiter struct {
	fallbackLimiter *timerate.Limiter
	redis           rediser
}

// A Limiter controls how frequently events are allowed to happen. It uses
// the redis to store data and fallbacks to the fallbackLimiter
// when Redis Server is not available.
func NewLimiter(redis rediser, fallbackLimiter *timerate.Limiter) *Limiter {
	return &Limiter{
		fallbackLimiter: fallbackLimiter,
		redis:           redis,
	}
}

// Allow reports whether an event with given name may happen at time now.
// It allows events up to rate maxRate within duration dur.
func (l *Limiter) Allow(name string, maxRate int64, dur time.Duration) (rate, reset int64, allow bool) {
	udur := int64(dur / time.Second)
	slot := time.Now().Unix() / udur
	name += strconv.FormatInt(slot, 10)
	reset = (slot + 1) * udur

	allow = l.fallbackLimiter.Allow()

	rate, err := l.increase(name, dur)
	if err == nil {
		allow = rate <= maxRate
	}

	return rate, reset, allow
}

func (l *Limiter) increase(name string, dur time.Duration) (int64, error) {
	name = redisPrefix + name
	var incr *redis.IntCmd
	_, err := l.redis.Pipelined(func(pipe *redis.Pipeline) error {
		incr = pipe.Incr(name)
		pipe.Expire(name, dur)
		return nil
	})

	rate, _ := incr.Result()
	return rate, err
}

// AllowMinute is shorthand for Allow(name, maxRate, time.Minute).
func (l *Limiter) AllowMinute(name string, maxRate int64) (int64, int64, bool) {
	return l.Allow(name, maxRate, time.Minute)
}

// AllowMinute is shorthand for Allow(name, maxRate, time.Hour).
func (l *Limiter) AllowHour(name string, maxRate int64) (int64, int64, bool) {
	return l.Allow(name, maxRate, time.Hour)
}
