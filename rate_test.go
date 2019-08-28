package redis_rate_test

import (
	"testing"
	"time"

	"github.com/go-redis/redis/v7"
	"github.com/stretchr/testify/assert"

	"github.com/go-redis/redis_rate/v7"
)

func rateLimiter(limit *redis_rate.Limit) *redis_rate.Limiter {
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{"server0": ":6379"},
	})
	if err := ring.FlushDb().Err(); err != nil {
		panic(err)
	}
	return redis_rate.NewLimiter(ring, limit)
}

func TestAllow(t *testing.T) {
	l := rateLimiter(&redis_rate.Limit{
		Burst:  10,
		Rate:   10,
		Period: time.Second,
	})

	res, err := l.Allow("test_id")
	assert.Nil(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, res.Remaining, 9)
	assert.Equal(t, res.RetryAfter, time.Duration(-1))
	assert.InDelta(t, res.ResetAfter, 100*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN("test_id", 2)
	assert.Nil(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, res.Remaining, 7)
	assert.Equal(t, res.RetryAfter, time.Duration(-1))
	assert.InDelta(t, res.ResetAfter, 300*time.Millisecond, float64(10*time.Millisecond))

	res, err = l.AllowN("test_id", 1000)
	assert.Nil(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, res.Remaining, 0)
	assert.InDelta(t, res.RetryAfter, 99*time.Second, float64(time.Second))
	assert.InDelta(t, res.ResetAfter, 300*time.Millisecond, float64(10*time.Millisecond))
}

func BenchmarkAllow(b *testing.B) {
	l := rateLimiter(&redis_rate.Limit{
		Burst:  1000,
		Rate:   1000,
		Period: time.Second,
	})

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := l.Allow("foo")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
