package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	timerate "golang.org/x/time/rate"

	"gopkg.in/go-redis/rate.v1"
	"gopkg.in/redis.v3"
)

func handler(w http.ResponseWriter, req *http.Request, rateLimiter *rate.Limiter) {
	limit := int64(5)
	userID := "user-12345"

	rate, reset, allowed := rateLimiter.AllowMinute(userID, limit)
	if !allowed {
		w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
		w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(limit-rate, 10))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		http.Error(w, "API rate limit exceeded.", 429)
		return
	}

	fmt.Fprintf(w, "Hello world!\n")
	fmt.Fprint(w, "Rate limit remaining: ", strconv.FormatInt(limit-rate, 10))
}

func Example_rateLimit() {
	ringOptions := &redis.RingOptions{Addrs: map[string]string{"1": "localhost:6379"}}
	ring := redis.NewRing(ringOptions)
	rateLimiter := NewRateLimiter(ring)

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		handler(w, req, rateLimiter)
	})

	http.HandleFunc("/favicon.ico", http.NotFound)
	http.ListenAndServe(":8080", nil)
}

func NewRateLimiter(ring *redis.Ring) *rate.Limiter {
	limiter := timerate.NewLimiter(timerate.Every(time.Second/100), 10)
	return rate.NewLimiter(ring, limiter)
}
