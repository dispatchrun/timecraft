package sandbox_test

import (
	"context"
	"io"
	"net"
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
	assert.Equal(t, n, ^wasi.Size(0))
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
		UserData:  42,
		EventType: wasi.FDReadEvent,
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
	assert.Equal(t, n, ^wasi.Size(0))
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
		UserData:  42,
		EventType: wasi.FDWriteEvent,
	}})

	n, errno = sys.FDWrite(ctx, 1, []wasi.IOVec{[]byte("Hello, World!")})
	assert.Equal(t, errno, wasi.ESUCCESS)
	assert.Equal(t, n, 13)
	assert.Equal(t, sys.FDClose(ctx, 1), wasi.ESUCCESS)
	assert.Equal(t, string(<-ch), "Hello, World!")
}

func TestPollUnsupportedClock(t *testing.T) {
	for _, clock := range []wasi.ClockID{wasi.Realtime, wasi.Monotonic, wasi.ProcessCPUTimeID, wasi.ThreadCPUTimeID} {
		t.Run(clock.String(), func(t *testing.T) {
			ctx := context.Background()
			sys := sandbox.New()

			subs := []wasi.Subscription{
				wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
					ID:      clock,
					Timeout: 10 * millis,
				}),
			}
			evs := make([]wasi.Event, len(subs))

			numEvents, errno := sys.PollOneOff(ctx, subs, evs)
			assert.Equal(t, numEvents, 1)
			assert.Equal(t, errno, wasi.ESUCCESS)
			assert.EqualAll(t, evs, []wasi.Event{{
				UserData:  42,
				Errno:     wasi.ENOTSUP,
				EventType: wasi.ClockEvent,
			}})
		})
	}
}

func TestPollTimeout(t *testing.T) {
	for _, clock := range []wasi.ClockID{wasi.Realtime, wasi.Monotonic} {
		t.Run(clock.String(), func(t *testing.T) {
			ctx := context.Background()
			sys := sandbox.New(sandbox.Time(time.Now))

			subs := []wasi.Subscription{
				wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
					ID:      clock,
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
		})
	}
}

func TestPollDeadline(t *testing.T) {
	for _, clock := range []wasi.ClockID{wasi.Realtime, wasi.Monotonic} {
		t.Run(clock.String(), func(t *testing.T) {
			ctx := context.Background()
			sys := sandbox.New(sandbox.Time(time.Now))

			timestamp, errno := sys.ClockTimeGet(ctx, clock, 1)
			assert.Equal(t, errno, wasi.ESUCCESS)

			subs := []wasi.Subscription{
				wasi.MakeSubscriptionClock(42, wasi.SubscriptionClock{
					ID:      clock,
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
		})
	}
}

func TestSystemListenPortZero(t *testing.T) {
	listen := sandbox.Listen(func(ctx context.Context, network, address string) (net.Listener, error) {
		return net.Listen(network, address)
	})

	ctx := context.Background()
	sys := sandbox.New(listen)

	lstn, err := sys.Listen(ctx, "tcp", "127.0.0.1:0")
	assert.OK(t, err)

	addr, ok := lstn.Addr().(*net.TCPAddr)
	assert.True(t, ok)
	assert.True(t, addr.IP.Equal(net.IPv4(127, 0, 0, 1)))
	assert.NotEqual(t, addr.Port, 0)
	assert.OK(t, lstn.Close())
}

func TestSystemListenAnyAddress(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	lstn, err := sys.Listen(ctx, "tcp", ":4242")
	assert.OK(t, err)

	addr, ok := lstn.Addr().(*net.TCPAddr)
	assert.True(t, ok)
	assert.True(t, addr.IP.Equal(net.IPv4(0, 0, 0, 0)))
	assert.Equal(t, addr.Port, 4242)
	assert.OK(t, lstn.Close())
}
