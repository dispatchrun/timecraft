package timecraft

import (
	"context"
	"math"
	"time"
)

func retry(ctx context.Context, maxAttempts int, minDelay, maxDelay time.Duration, fn func() bool) {
	for attempt := 1; fn() && attempt < maxAttempts; attempt++ {
		delay := minDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		if delay > maxDelay {
			delay = maxDelay
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}
