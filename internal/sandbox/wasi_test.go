package sandbox_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/wasitest"
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

	n, errno := sys.FDWrite(ctx, 1, []wasi.IOVec{[]byte("Hello, World!")})
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
					ID:        clock,
					Timeout:   10 * millis,
					Precision: 10 * millis,
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

func TestSandboxWASI(t *testing.T) {
	wasitest.TestSystem(t, func(config wasitest.TestConfig) (wasi.System, error) {
		options := []sandbox.Option{
			sandbox.Args(config.Args...),
			sandbox.Environ(config.Environ...),
			sandbox.Rand(config.Rand),
			sandbox.Time(config.Now),
			sandbox.MaxOpenFiles(config.MaxOpenFiles),
			sandbox.MaxOpenDirs(config.MaxOpenDirs),
			sandbox.Network(sandbox.Host()),
		}

		if config.RootFS != "" {
			options = append(options, rootFS(config.RootFS))
		}

		sys := sandbox.New(options...)

		stdin, stdout, stderr := sys.Stdin(), sys.Stdout(), sys.Stderr()
		go copyAndClose(stdin, config.Stdin)
		go copyAndClose(config.Stdout, stdout)
		go copyAndClose(config.Stderr, stderr)

		return sys, nil
		//return wasi.Trace(os.Stderr, sys), nil
	})
}

func copyAndClose(w io.WriteCloser, r io.ReadCloser) {
	if w != nil {
		defer w.Close()
	}
	if r != nil {
		defer r.Close()
	}
	if w != nil && r != nil {
		_, _ = io.Copy(w, r)
	}
}
