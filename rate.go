package rate

import (
	"strconv"
	"time"

	timerate "golang.org/x/time/rate"

	"gopkg.in/redis.v3"
)

const redisPrefix = "rate:"

type Limiter struct {
	limiter *timerate.Limiter
	ring    *redis.Ring
}

func NewLimiter(ring *redis.Ring, limiter *timerate.Limiter) *Limiter {
	return &Limiter{
		limiter: limiter,
		ring:    ring,
	}
}

func (l *Limiter) Allow(
	name string, limit int64, dur time.Duration,
) (rate, reset int64, allow bool) {
	udur := int64(dur / time.Second)
	slot := time.Now().Unix() / udur
	name += strconv.FormatInt(slot, 10)
	reset = (slot + 1) * udur

	allow = l.limiter.Allow()

	rate, err := l.increase(name, dur)
	if err == nil {
		allow = rate <= limit
	}

	return rate, reset, allow
}

func (l *Limiter) increase(name string, dur time.Duration) (int64, error) {
	var incr *redis.IntCmd
	_, err := l.ring.Pipelined(func(pipe *redis.RingPipeline) error {
		key := redisPrefix + name
		incr = pipe.Incr(key)
		pipe.Expire(key, dur)
		return nil
	})

	rate, _ := incr.Result()
	return rate, err
}

func (l *Limiter) AllowMinute(name string, limit int64) (int64, int64, bool) {
	return l.Allow(name, limit, time.Minute)
}

func (l *Limiter) AllowHour(name string, limit int64) (int64, int64, bool) {
	return l.Allow(name, limit, time.Hour)
}
