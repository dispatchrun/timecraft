package sandbox

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stealthrocket/wasi-go"
)

type channel chan []byte

func (ch channel) poll(ctx context.Context, flags wasi.FDFlags, timeout <-chan time.Time, done <-chan struct{}) ([]byte, wasi.Errno) {
	if flags.Has(wasi.NonBlock) {
		select {
		case data := <-ch:
			return data, wasi.ESUCCESS
		case <-done:
			return nil, wasi.EBADF
		default:
			return nil, wasi.EAGAIN
		}
	} else {
		select {
		case data := <-ch:
			return data, wasi.ESUCCESS
		case <-done:
			return nil, wasi.EBADF
		case <-timeout:
			return nil, wasi.ETIMEDOUT
		case <-ctx.Done():
			return nil, wasi.MakeErrno(context.Cause(ctx))
		}
	}
}

func (ch channel) read(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ev *event, timeout <-chan time.Time, done <-chan struct{}) (size wasi.Size, errno wasi.Errno) {
	data, errno := ch.poll(ctx, flags, timeout, done)
	if errno != wasi.ESUCCESS {
		// Make a special case for the error indicating that the done channel
		// was closed, it must return size=0 and errno=0 to indicate EOF.
		if errno == wasi.EBADF {
			errno = wasi.ESUCCESS
		} else {
			size = ^wasi.Size(0)
		}
		return size, errno
	}
	for _, iov := range iovs {
		n := copy(iov, data)
		data = data[n:]
		size += wasi.Size(n)
		if len(data) == 0 {
			break
		}
	}
	ev.clear()
	ch <- data
	return size, wasi.ESUCCESS
}

func (ch channel) write(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ev *event, timeout <-chan time.Time, done <-chan struct{}) (size wasi.Size, errno wasi.Errno) {
	data, errno := ch.poll(ctx, flags, timeout, done)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	for _, iov := range iovs {
		n := copy(data, iov)
		data = data[n:]
		size += wasi.Size(n)
		if len(data) == 0 {
			break
		}
	}
	ev.clear()
	ch <- data
	return size, wasi.ESUCCESS
}

type event struct {
	lock   *sync.Mutex
	status atomic.Uint32
	signal chan<- struct{}
}

const (
	cleared uint32 = iota
	ready
	aborted
)

func makeEvent(lock *sync.Mutex) event {
	return event{lock: lock}
}

func (ev *event) state() uint32 {
	return ev.status.Load()
}

func (ev *event) abort() {
	// Abort the event, causing it to signal its hook (if it has one) and
	// preventing it from going back to be ready or cleared. After entering
	// this state, the event will always trigger if polled.
	ev.status.Store(aborted)
	ev.trigger()
}

func (ev *event) clear() {
	// Clear the event state, which means moving it from ready to clear.
	// If the event is in the abort state, this function has no effect
	// since it is not possible to bring an event back from being aborted.
	ev.status.CompareAndSwap(ready, cleared)
}

func (ev *event) poll(signal chan<- struct{}) bool {
	if ev.status.Load() != cleared {
		ev.signal = nil
		return true
	} else {
		ev.signal = signal
		return false
	}
}

func (ev *event) trigger() {
	ev.synchronize(func() {
		ev.status.CompareAndSwap(cleared, ready)
		trigger(ev.signal)
	})
}

func (ev *event) update(trigger bool) {
	if trigger {
		ev.trigger()
	} else {
		ev.clear()
	}
}

func (ev *event) synchronize(f func()) {
	if ev.lock != nil {
		ev.lock.Lock()
		defer ev.lock.Unlock()
	}
	f()
}

func trigger(signal chan<- struct{}) {
	if signal != nil {
		select {
		case signal <- struct{}{}:
		default:
		}
	}
}

// pipe is a unidirectional channel allowing data to pass between the host and
// a guest module.
type pipe struct {
	defaultFile
	flags wasi.FDFlags
	mu    sync.Mutex
	ch    channel
	ev    event
	once  sync.Once
	done  chan struct{}
}

func newPipe(lock *sync.Mutex) *pipe {
	return &pipe{
		ch:   make(channel),
		ev:   makeEvent(lock),
		done: make(chan struct{}),
	}
}

func (p *pipe) close() {
	p.once.Do(func() { close(p.done); p.ev.abort() })
}

func (p *pipe) FDClose(ctx context.Context) wasi.Errno {
	p.close()
	return wasi.ESUCCESS
}

func (p *pipe) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	p.flags = flags
	return wasi.ESUCCESS
}

// input allows data to flow from the host to the guest.
type input struct{ *pipe }

func (in input) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return in.ch.read(ctx, iovs, in.flags, &in.ev, nil, in.done)
}

func (in input) FDPoll(ev wasi.EventType, ch chan<- struct{}) bool {
	return ev == wasi.FDReadEvent && in.ev.poll(ch)
}

type inputReadCloser struct {
	in *pipe
}

func (r inputReadCloser) Close() error {
	r.in.close()
	return nil
}

func (r inputReadCloser) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	ctx := context.Background()
	flag := wasi.FDFlags(0) // blocking
	iovs := []wasi.IOVec{b}
	size, errno := r.in.ch.read(ctx, iovs, flag, &r.in.ev, nil, r.in.done)
	if errno != wasi.ESUCCESS {
		return int(size), errno
	}
	if size == 0 {
		return 0, io.EOF
	}
	return int(size), nil
}

type inputWriteCloser struct {
	in *pipe
}

func (w inputWriteCloser) Close() error {
	w.in.close()
	return nil
}

func (w inputWriteCloser) Write(b []byte) (int, error) {
	w.in.mu.Lock()
	defer w.in.mu.Unlock()

	n := 0
	for n < len(b) {
		w.in.ev.trigger()
		select {
		case w.in.ch <- b[n:]:
			n = len(b) - len(<-w.in.ch)
		case <-w.in.done:
			return n, io.ErrClosedPipe
		}
	}
	return n, nil
}

// output allows data to flow from the guest to the host.
type output struct{ *pipe }

func (out output) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return out.ch.write(ctx, iovs, out.flags, &out.ev, nil, out.done)
}

func (out output) FDPoll(ev wasi.EventType, ch chan<- struct{}) bool {
	return ev == wasi.FDWriteEvent && out.ev.poll(ch)
}

type outputReadCloser struct {
	out *pipe
}

func (r outputReadCloser) Close() error {
	r.out.close()
	return nil
}

func (r outputReadCloser) Read(b []byte) (int, error) {
	r.out.mu.Lock()
	defer r.out.mu.Unlock()

	r.out.ev.trigger()
	select {
	case r.out.ch <- b:
		return len(b) - len(<-r.out.ch), nil
	case <-r.out.done:
		return 0, io.EOF
	}
}

type outputWriteCloser struct {
	out *pipe
}

func (w outputWriteCloser) Close() error {
	w.out.close()
	return nil
}

func (w outputWriteCloser) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	ctx := context.Background()
	flag := wasi.FDFlags(0) // blocking
	for n < len(b) {
		iovs := []wasi.IOVec{b[n:]}
		size, errno := w.out.ch.write(ctx, iovs, flag, &w.out.ev, nil, w.out.done)
		n += int(size)
		if errno != wasi.ESUCCESS {
			return n, errno
		}
	}
	return n, nil
}
