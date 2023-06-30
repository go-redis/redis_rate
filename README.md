# Rate limiting for go-redis

[![PkgGoDev](https://pkg.go.dev/badge/github.com/ductone/redis_rate/v11)](https://pkg.go.dev/github.com/ductone/redis_rate/v11)

This package is a fork of [go-redis/redis_rate](https://github.com/go-redis/redis_rate).

This `go-redis/redis_rate` was based on [rwz/redis-gcra](https://github.com/rwz/redis-gcra) and
implements [GCRA](https://en.wikipedia.org/wiki/Generic_cell_rate_algorithm) (aka leaky bucket) for
rate limiting based on Redis. The code requires Redis version 3.2 or newer since it relies on
[replicate_commands](https://redis.io/commands/eval#replicating-commands-instead-of-scripts)
feature.

## Fork Features

- _Allow Multi_: Check if you can allow multiple requests at once in a single Redis pipelined call.
- General cleanup for modern Go.

## Example

```go
package redis_rate_test

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/ductone/redis_rate/v11"
)

func ExampleNewLimiter() {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	_ = rdb.FlushDB(ctx).Err()

	limiter := redis_rate.NewLimiter(rdb)
	res, err := limiter.Allow(ctx, "project:123", redis_rate.PerSecond(10))
	if err != nil {
		panic(err)
	}
	fmt.Println("allowed", res.Allowed, "remaining", res.Remaining)
	// Output: allowed 1 remaining 9
}
```
