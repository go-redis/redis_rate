package rate_test

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	timerate "golang.org/x/time/rate"

	"gopkg.in/go-redis/rate.v4"
	"gopkg.in/redis.v4"
)

func handler(w http.ResponseWriter, req *http.Request, rateLimiter *rate.Limiter) {
	userID := "user-12345"
	limit := int64(5)

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
	ring := redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": "localhost:6379",
		},
	})
	fallbackLimiter := timerate.NewLimiter(timerate.Every(time.Second), 100)
	limiter := rate.NewLimiter(ring, fallbackLimiter)

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		handler(w, req, limiter)
	})

	http.HandleFunc("/favicon.ico", http.NotFound)
	http.ListenAndServe(":8080", nil)
}
