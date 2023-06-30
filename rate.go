package redis_rate //nolint:revive // upstream used this name

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRedisPrefix = "rate:"

type RedisClientConn interface {
	Pipelined(ctx context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)

	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd
	ScriptExists(ctx context.Context, hashes ...string) *redis.BoolSliceCmd
	ScriptLoad(ctx context.Context, script string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd

	EvalRO(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalShaRO(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd

	// redis.Cmdable // can uncomment when testing using new interface methods
}

type Limit struct {
	Rate   int
	Burst  int
	Period time.Duration
}

func (l Limit) String() string {
	return fmt.Sprintf("%d req/%s (burst %d)", l.Rate, fmtDur(l.Period), l.Burst)
}

func (l Limit) IsZero() bool {
	return l == Limit{}
}

func fmtDur(d time.Duration) string {
	switch d {
	case time.Second:
		return "s"
	case time.Minute:
		return "m"
	case time.Hour:
		return "h"
	default:
		return d.String()
	}
}

func PerSecond(rate int) Limit {
	return Limit{
		Rate:   rate,
		Period: time.Second,
		Burst:  rate,
	}
}

func PerMinute(rate int) Limit {
	return Limit{
		Rate:   rate,
		Period: time.Minute,
		Burst:  rate,
	}
}

func PerHour(rate int) Limit {
	return Limit{
		Rate:   rate,
		Period: time.Hour,
		Burst:  rate,
	}
}

// ------------------------------------------------------------------------------

// Limiter controls how frequently events are allowed to happen.
type Limiter struct {
	rdb    RedisClientConn
	prefix string
}

// NewLimiter returns a new Limiter.
func NewLimiter(rdb RedisClientConn, prefix string) *Limiter {
	if prefix == "" {
		prefix = defaultRedisPrefix
	}
	return &Limiter{
		rdb:    rdb,
		prefix: prefix,
	}
}

func (l *Limiter) LoadScripts(ctx context.Context) error {
	_, err := allowN.Load(ctx, l.rdb).Result()
	if err != nil {
		return err
	}

	_, err = allowAtMost.Load(ctx, l.rdb).Result()
	if err != nil {
		return err
	}

	return nil
}

// Allow is a shortcut for AllowN(ctx, key, limit, 1).
func (l *Limiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	return l.AllowN(ctx, key, limit, 1)
}

type pipelineResult struct {
	key   string
	limit Limit
	cmd   *redis.Cmd
}

var ErrAllowMultiScriptFailed = errors.New("redis_rate: invalid result from SCRIPT EXISTS in allow multi")
var ErrAllowMultiTooManyRetries = errors.New("redis_rate: allow multi too many retries to load scripts")

func (l *Limiter) AllowMulti(ctx context.Context, limits map[string]Limit) ([]*Result, error) {
	return l.allowMulti(ctx, limits, 0)
}

func (l *Limiter) allowMulti(ctx context.Context, limits map[string]Limit, depth int) ([]*Result, error) {
	if depth > 10 {
		return nil, ErrAllowMultiTooManyRetries
	}

	var existsCmd *redis.BoolSliceCmd
	results := make([]*pipelineResult, 0, len(limits))
	buf := bytes.Buffer{}
	_, err := l.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		existsCmd = allowN.Exists(ctx, pipe)
		for key, limit := range limits {
			values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), int(1)}

			buf.Reset()
			_, _ = buf.WriteString(l.prefix)
			_, _ = buf.WriteString(key)

			results = append(results, &pipelineResult{
				key:   key,
				limit: limit,
				cmd: allowN.EvalSha(
					ctx,
					pipe,
					[]string{buf.String()},
					values...,
				),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	exists, err := existsCmd.Result()
	if err != nil {
		return nil, err
	}
	if len(exists) != 1 {
		return nil, ErrAllowMultiScriptFailed
	}

	if !exists[0] {
		err = l.LoadScripts(ctx)
		if err != nil {
			return nil, err
		}
		return l.allowMulti(ctx, limits, depth+1)
	}

	rv := make([]*Result, 0, len(results))
	for _, result := range results {
		v, err := result.cmd.Result()
		if err != nil {
			return nil, err
		}
		values := v.([]interface{})
		rr, err := parseScriptResult(result.key, result.limit, values)
		if err != nil {
			return nil, err
		}
		rv = append(rv, rr)
	}

	return rv, nil
}

func parseScriptResult(key string, limit Limit, values []interface{}) (*Result, error) {
	retryAfter, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return nil, err
	}

	resetAfter, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Key:        key,
		Limit:      limit,
		Allowed:    values[0].(int64),
		Remaining:  values[1].(int64),
		Used:       0,
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

// AllowN reports whether n events may happen at time now.
func (l *Limiter) AllowN(
	ctx context.Context,
	key string,
	limit Limit,
	n int,
) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowN.Run(ctx, l.rdb, []string{l.prefix + key}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	return parseScriptResult(key, limit, values)
}

// AllowAtMost reports whether at most n events may happen at time now.
// It returns number of allowed events that is less than or equal to n.
func (l *Limiter) AllowAtMost(
	ctx context.Context,
	key string,
	limit Limit,
	n int,
) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowAtMost.Run(ctx, l.rdb, []string{l.prefix + key}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	return parseScriptResult(key, limit, values)
}

// Reset gets a key and reset all limitations and previous usages.
func (l *Limiter) Reset(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, l.prefix+key).Err()
}

func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}

type Result struct {
	// Name of the key used for this result.
	Key string

	// Limit is the limit that was used to obtain this result.
	Limit Limit

	// Allowed is the number of events that may happen at time now.
	Allowed int64

	// Used is the number of events that have already happened at time now.
	Used int64

	// Remaining is the maximum number of requests that could be
	// permitted instantaneously for this key given the current
	// state. For example, if a rate limiter allows 10 requests per
	// second and has already received 6 requests for this key this
	// second, Remaining would be 4.
	Remaining int64

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
