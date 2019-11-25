package redis_rate

import (
	"strconv"
	"time"

	"github.com/go-redis/redis/v7"
)

const redisPrefix = "rate:"

type rediser interface {
	Eval(script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(sha1 string, keys []string, args ...interface{}) *redis.Cmd
	ScriptExists(hashes ...string) *redis.BoolSliceCmd
	ScriptLoad(script string) *redis.StringCmd
}

type Limit struct {
	Rate   int
	Period time.Duration
	Burst  int
}

// Limiter controls how frequently events are allowed to happen.
type Limiter struct {
	rdb   rediser
	limit *Limit
}

// NewLimiter returns a new Limiter that allows events up to rate r
// and permits bursts of at most b tokens.
func NewLimiter(rdb rediser, limit *Limit) *Limiter {
	return &Limiter{
		rdb:   rdb,
		limit: limit,
	}
}

// Allow is shorthand for AllowN(key, 1).
func (l *Limiter) Allow(key string) (*Result, error) {
	return l.AllowN(key, 1)
}

// AllowN reports whether n events may happen at time now.
func (l *Limiter) AllowN(key string, n int) (*Result, error) {
	values := []interface{}{l.limit.Burst, l.limit.Rate, l.limit.Period.Seconds(), n}
	v, err := gcra.Run(l.rdb, []string{redisPrefix + key}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	retryAfter, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return nil, err
	}

	resetAfter, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Limit:      l.limit.Rate,
		Allowed:    values[0].(int64) == 0,
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}

type Result struct {
	//Limit reports the available limit
	Limit int
	
	// Allowed reports whether event may happen at time now.
	Allowed bool

	// Remaining is the maximum number of requests that could be
	// permitted instantaneously for this key given the current
	// state. For example, if a rate limiter allows 10 requests per
	// second and has already received 6 requests for this key this
	// second, Remaining would be 4.
	Remaining int

	// RetryAfter is the time until the next request will be permitted.
	// It should be -1 unless the rate limit has been exceeded.
	RetryAfter time.Duration

	// ResetAfter is the time until the RateLimiter returns to its
	// initial state for a given key. For example, if a rate limiter
	// manages requests per second and received one request 200ms ago,
	// Reset would return 800ms. You can also think of this as the time
	// until Limit and Remaining will be equal.
	ResetAfter time.Duration
}
