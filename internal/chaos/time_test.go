package chaos_test

import (
	"context"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/chaos"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/wasi-go"
)

func TestClockDrift(t *testing.T) {
	epoch := time.Now()

	currentTime := epoch
	timeFunc := func() time.Time { return currentTime }

	sys := chaos.ClockDrift(sandbox.New(sandbox.Time(timeFunc)))
	ctx := context.Background()

	timeline := make([]wasi.Timestamp, 120*1e3)

	computeDrift := func(timestamp wasi.Timestamp) time.Duration {
		drift := time.Duration(currentTime.Sub(time.Unix(0, int64(timestamp))))
		if drift < 0 {
			drift = -drift
		}
		return drift
	}

	for i := range timeline {
		timestamp, errno := sys.ClockTimeGet(ctx, wasi.Realtime, 1)
		assert.Equal(t, errno, wasi.ESUCCESS)

		drift := computeDrift(timestamp)
		if drift > currentTime.Sub(epoch) {
			t.Fatalf("drift cannot be greater than total duration: drift=%s duration=%s", drift, currentTime.Sub(epoch))
		}

		timeline[i] = timestamp
		currentTime = currentTime.Add(time.Millisecond)
	}

	drift := computeDrift(timeline[len(timeline)-1])
	t.Logf("drift after %s = %s\n", currentTime.Sub(epoch), drift)
}

func BenchmarkClockDrift(b *testing.B) {
	epoch := time.Now()

	currentTime := epoch
	timeFunc := func() time.Time { return currentTime }

	sys := chaos.ClockDrift(sandbox.New(sandbox.Time(timeFunc)))
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_, errno := sys.ClockTimeGet(ctx, wasi.Realtime, 1)
		if errno != wasi.ESUCCESS {
			b.Fatal(errno)
		}
		currentTime = currentTime.Add(time.Millisecond)
	}
}
