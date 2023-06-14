package sandbox

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/wasi-go"
)

type channel chan []byte

func (ch channel) send(data []byte, done <-chan struct{}) bool {
	select {
	case ch <- data:
		return true
	case <-done:
		return false
	}
}

func (ch channel) wait() []byte {
	return <-ch
}

func (ch channel) poll(ctx context.Context, flags wasi.FDFlags, done <-chan struct{}) ([]byte, wasi.Errno) {
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
		case <-ctx.Done():
			return nil, wasi.MakeErrno(context.Cause(ctx))
		}
	}
}

func (ch channel) read(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ev *event, done <-chan struct{}) (size wasi.Size, errno wasi.Errno) {
	data, errno := ch.poll(ctx, flags, done)
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

func (ch channel) write(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ev *event, done <-chan struct{}) (size wasi.Size, errno wasi.Errno) {
	data, errno := ch.poll(ctx, flags, done)
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

type pipe struct {
	defaultFile
	flags wasi.FDFlags
	// Pointer to mutex synchronizing poll events.
	pmu *sync.Mutex
	// Read-end of the pipe (host side).
	rmu sync.Mutex
	rch channel
	rev event
	// Write-end of the pipe (host side).
	wmu sync.Mutex
	wch channel
	wev event
	// The once value guards the closure of the `done` event.
	once sync.Once
	done chan struct{}
}

func newPipe(pmu *sync.Mutex) *pipe {
	return &pipe{
		pmu:  pmu,
		rch:  make(channel),
		wch:  make(channel),
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

func (p *pipe) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return p.wch.read(ctx, iovs, p.flags, &p.wev, p.done)
}

func (p *pipe) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return p.rch.write(ctx, iovs, p.flags, &p.rev, p.done)
}

func (p *pipe) Close() error {
	p.close()
	return nil
}

func (p *pipe) Read(b []byte) (int, error) {
	p.rmu.Lock()
	defer p.rmu.Unlock()

	p.rev.trigger(p.pmu)
	if !p.rch.send(b, p.done) {
		return 0, io.EOF
	}
	r := p.rch.wait()
	return len(b) - len(r), nil
}

func (p *pipe) Write(b []byte) (int, error) {
	p.wmu.Lock()
	defer p.wmu.Unlock()

	n := 0
	for n < len(b) {
		p.wev.trigger(p.pmu)
		if !p.wch.send(b[n:], p.done) {
			return n, io.ErrClosedPipe
		}
		r := p.wch.wait()
		n = len(b) - len(r)
	}
	return n, nil
}

func (p *pipe) Hook(ev wasi.EventType, ch chan<- struct{}) {
	switch ev {
	case wasi.FDReadEvent:
		p.wev.hook(ch)
	case wasi.FDWriteEvent:
		p.rev.hook(ch)
	}
}

func (p *pipe) Poll(ev wasi.EventType) bool {
	switch ev {
	case wasi.FDReadEvent:
		return p.wev.poll()
	case wasi.FDWriteEvent:
		return p.rev.poll()
	default:
		return false
	}
}
