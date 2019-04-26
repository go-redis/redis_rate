package redis_rate

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

var limitPeriodTests = []struct {
	rate   rate.Limit
	limit  int64
	period time.Duration
}{
	{rate.Every(time.Millisecond), 1000, time.Second},
	{rate.Every(time.Second), 1, time.Second},
	{rate.Every(time.Hour / 5), 1, 12 * time.Minute},
}

func TestLimitPeriod(t *testing.T) {
	for i, test := range limitPeriodTests {
		limit, period := limitPeriod(test.rate)
		if limit != test.limit {
			t.Fatalf("%d: got %d, wanted %d", i, limit, test.limit)
		}
		if period != test.period {
			t.Fatalf("%d: got %s, wanted %s", i, period, test.period)
		}
	}
}
