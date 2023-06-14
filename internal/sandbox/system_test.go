package sandbox_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/wasi-go"
)

const (
	millis = 1000 * micros
	micros = 1000 * nanos
	nanos  = 1
)

func TestPollNothing(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	numEvents, errno := sys.PollOneOff(ctx, nil, nil)
	assert.Equal(t, numEvents, 0)
	assert.Equal(t, errno, wasi.EINVAL)
}

func TestPollUnknownFile(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionFDReadWrite(42, wasi.FDReadEvent, wasi.SubscriptionFDReadWrite{FD: 1234}),
	}
	evs := make([]wasi.Event, len(subs))

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:  42,
		Errno:     wasi.EBADF,
		EventType: wasi.FDReadEvent,
	}})
}

func TestPollStdin(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	errno := sys.FDStatSetFlags(ctx, 0, wasi.NonBlock)
	assert.Equal(t, errno, wasi.ESUCCESS)

	buffer := make([]byte, 32)
	n, errno := sys.FDRead(ctx, 0, []wasi.IOVec{buffer})
	assert.Equal(t, n, 0)
	assert.Equal(t, errno, wasi.EAGAIN)

	go func() {
		n, err := io.WriteString(sys.Stdin(), "Hello, World!")
		assert.OK(t, err)
		assert.Equal(t, n, 13)
	}()

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionFDReadWrite(42, wasi.FDReadEvent, wasi.SubscriptionFDReadWrite{FD: 0}),
	}
	evs := make([]wasi.Event, len(subs))

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:    42,
		EventType:   wasi.FDReadEvent,
		FDReadWrite: wasi.EventFDReadWrite{NBytes: 1},
	}})

	n, errno = sys.FDRead(ctx, 0, []wasi.IOVec{buffer})
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.Equal(t, n, 13)
	assert.Equal(t, string(buffer[:n]), "Hello, World!")
}

func TestPollStdout(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	errno := sys.FDStatSetFlags(ctx, 1, wasi.NonBlock)
	assert.Equal(t, errno, wasi.ESUCCESS)

	n, errno := sys.FDWrite(ctx, 1, []wasi.IOVec{[]byte("1")})
	assert.Equal(t, n, 0)
	assert.Equal(t, errno, wasi.EAGAIN)

	ch := make(chan []byte)
	go func() {
		b, err := io.ReadAll(sys.Stdout())
		assert.OK(t, err)
		ch <- b
	}()

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionFDReadWrite(42, wasi.FDWriteEvent, wasi.SubscriptionFDReadWrite{FD: 1}),
	}
	evs := make([]wasi.Event, len(subs))

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:    42,
		EventType:   wasi.FDWriteEvent,
		FDReadWrite: wasi.EventFDReadWrite{NBytes: 1},
	}})

	n, errno = sys.FDWrite(ctx, 1, []wasi.IOVec{[]byte("Hello, World!")})
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.Equal(t, n, 13)
	assert.Equal(t, sys.FDClose(ctx, 1), wasi.ESUCCESS)
	assert.Equal(t, string(<-ch), "Hello, World!")
}

func TestPollTimeoutRealtime(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New(sandbox.Time(time.Now))

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
			ID:      wasi.Realtime,
			Timeout: 10 * millis,
		}),
	}
	evs := make([]wasi.Event, len(subs))
	now := time.Now()

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.True(t, time.Since(now) > 10*millis)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:  42,
		EventType: wasi.ClockEvent,
	}})
}

func TestPollTimeoutMonotonic(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New(sandbox.Time(time.Now))

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
			ID:      wasi.Monotonic,
			Timeout: 10 * millis,
		}),
	}
	evs := make([]wasi.Event, len(subs))
	now := time.Now()

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.True(t, time.Since(now) > 10*millis)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:  42,
		EventType: wasi.ClockEvent,
	}})
}

func TestPollDeadlineRealtime(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New(sandbox.Time(time.Now))

	timestamp, errno := sys.ClockTimeGet(ctx, wasi.Realtime, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
			ID:      wasi.Realtime,
			Timeout: timestamp + 10*millis,
			Flags:   wasi.Abstime,
		}),
	}
	evs := make([]wasi.Event, len(subs))
	now := time.Now()

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.True(t, time.Since(now) > 10*millis)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:  42,
		EventType: wasi.ClockEvent,
	}})
}

func TestPollDeadlineMonotonic(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New(sandbox.Time(time.Now))

	timestamp, errno := sys.ClockTimeGet(ctx, wasi.Monotonic, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)

	subs := []wasi.Subscription{
		wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
			ID:      wasi.Monotonic,
			Timeout: timestamp + 10*millis,
			Flags:   wasi.Abstime,
		}),
	}
	evs := make([]wasi.Event, len(subs))
	now := time.Now()

	numEvents, errno := sys.PollOneOff(ctx, subs, evs)
	assert.True(t, time.Since(now) > 10*millis)
	assert.Equal(t, numEvents, 1)
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.EqualAll(t, evs, []wasi.Event{{
		UserData:  42,
		EventType: wasi.ClockEvent,
	}})
}
