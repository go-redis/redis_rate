package redis_rate_test

import (
	"fmt"

	"github.com/go-redis/redis/v7"
	"github.com/go-redis/redis_rate/v8"
)

func ExampleNewLimiter() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	_ = rdb.FlushDB().Err()

	limiter := redis_rate.NewLimiter(rdb)
	res, err := limiter.Allow("project:123", redis_rate.PerSecond(10))
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Allowed, res.Remaining)
	// Output: true 9
}
