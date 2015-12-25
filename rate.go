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
	incr    func(string, time.Duration) int64
}

func NewLimiter(incr func(string, time.Duration) int64, limiter *timerate.Limiter) *Limiter {
	return &Limiter{
		limiter: limiter,
		incr:    incr,
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

	rate = l.incr(name, dur)
	if err == nil {
		allow = rate <= limit
	}

	return rate, reset, allow
}

func (l *Limiter) AllowMinute(name string, limit int64) (int64, int64, bool) {
	return l.Allow(name, limit, time.Minute)
}

func (l *Limiter) AllowHour(name string, limit int64) (int64, int64, bool) {
	return l.Allow(name, limit, time.Hour)
}

func Increase(name string, dur time.Duration) {
	ringOptions := &redis.RingOptions{Addrs: map[string]string{"1": "localhost:6379"}}
	ring := redis.NewRing(ringOptions)

	var incr *redis.IntCmd
	_, err := ring.Pipelined(func(pipe *redis.RingPipeline) error {
		key := redisPrefix + name
		incr = pipe.Incr(key)
		pipe.Expire(key, dur)
		return nil
	})
	rate, _ = incr.Result()

	return rate
}
