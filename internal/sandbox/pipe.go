package sandbox

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/wasi-go"
)

type channel chan []byte

func (ch channel) poll(ctx context.Context, flags wasi.FDFlags, timeout, done <-chan struct{}) ([]byte, wasi.Errno) {
	if flags.Has(wasi.NonBlock) {
		select {
		case data := <-ch:
			return data, wasi.ESUCCESS
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

func (ch channel) read(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ev *event, timeout, done <-chan struct{}) (size wasi.Size, errno wasi.Errno) {
	data, errno := ch.poll(ctx, flags, timeout, done)
	if errno != wasi.ESUCCESS {
		// Make a special case for the error indicating that the done channel
		// was closed, it must return size=0 and errno=0 to indicate EOF.
		if errno == wasi.EBADF {
			errno = wasi.ESUCCESS
		}
		return 0, errno
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

func (ch channel) write(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ev *event, timeout, done <-chan struct{}) (size wasi.Size, errno wasi.Errno) {
	data, errno := ch.poll(ctx, flags, timeout, done)
	if errno != wasi.ESUCCESS {
		return 0, errno
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
	ready  atomic.Bool
	signal chan<- struct{}
}

func (ev *event) clear() {
	ev.ready.Store(false)
}

func (ev *event) poll() bool {
	ev.signal = nil
	return ev.ready.Swap(false)
}

func (ev *event) hook(signal chan<- struct{}) {
	if !ev.ready.Load() {
		ev.signal = signal
	} else if signal != nil {
		select {
		case signal <- struct{}{}:
		default:
		}
	}
}

func (ev *event) trigger(mu *sync.Mutex) {
	mu.Lock()
	ev.ready.Store(true)
	if ev.signal != nil {
		select {
		case ev.signal <- struct{}{}:
		default:
		}
		ev.signal = nil
	}
	mu.Unlock()
}

// pipe is a unidirectional channel allowing data to pass between the host and
// a guest module.
type pipe struct {
	defaultFile
	flags wasi.FDFlags
	lock  *sync.Mutex
	mu    sync.Mutex
	ch    channel
	ev    event
	once  sync.Once
	done  chan struct{}
}

func newPipe(lock *sync.Mutex) *pipe {
	return &pipe{
		lock: lock,
		ch:   make(channel),
		done: make(chan struct{}),
	}
}

func (p *pipe) close() {
	p.once.Do(func() { close(p.done) })
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

func (in input) FDHook(ev wasi.EventType, ch chan<- struct{}) {
	if ev == wasi.FDReadEvent {
		in.ev.hook(ch)
	}
}

func (in input) FDPoll(ev wasi.EventType) bool {
	return ev == wasi.FDReadEvent && in.ev.poll()
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
		w.in.ev.trigger(w.in.lock)
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

func (out output) FDHook(ev wasi.EventType, ch chan<- struct{}) {
	if ev == wasi.FDWriteEvent {
		out.ev.hook(ch)
	}
}

func (out output) FDPoll(ev wasi.EventType) bool {
	return ev == wasi.FDWriteEvent && out.ev.poll()
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

	r.out.ev.trigger(r.out.lock)
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
