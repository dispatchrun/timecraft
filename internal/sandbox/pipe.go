package sandbox

import (
	"context"
	"io"
	"sync"

	"github.com/stealthrocket/wasi-go"
)

type channel chan []byte

func (ch channel) send(data []byte, ready, done event) bool {
	ready.set()
	select {
	case ch <- data:
		return true
	case <-done:
		ready.clear()
		return false
	}
}

func (ch channel) wait() []byte {
	return <-ch
}

func (ch channel) poll(ctx context.Context, flags wasi.FDFlags, done event) ([]byte, wasi.Errno) {
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

func (ch channel) read(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ready, done event) (size wasi.Size, errno wasi.Errno) {
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
	ready.clear()
	ch <- data
	return size, wasi.ESUCCESS
}

func (ch channel) write(ctx context.Context, iovs []wasi.IOVec, flags wasi.FDFlags, ready, done event) (size wasi.Size, errno wasi.Errno) {
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
	ready.clear()
	ch <- data
	return size, wasi.ESUCCESS
}

type event chan struct{}

func (ev event) set() {
	select {
	case ev <- struct{}{}:
	default:
	}
}

func (ev event) clear() bool {
	select {
	case <-ev:
		return true
	default:
		return false
	}
}

type pipe struct {
	defaultFile
	flags wasi.FDFlags
	// Read-end of the pipe.
	rmu sync.Mutex
	rch channel
	rev event
	// Write-end of the pipe.
	wmu sync.Mutex
	wch channel
	wev event
	// The once value guards the closure of the `done` event.
	once sync.Once
	done event
}

func makePipe() pipe {
	return pipe{
		rch:  make(channel),
		wch:  make(channel),
		rev:  make(event),
		wev:  make(event),
		done: make(event),
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
	return p.wch.read(ctx, iovs, p.flags, p.wev, p.done)
}

func (p *pipe) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return p.rch.write(ctx, iovs, p.flags, p.rev, p.done)
}

func (p *pipe) Close() error {
	p.close()
	return nil
}

func (p *pipe) Read(b []byte) (int, error) {
	p.rmu.Lock()
	defer p.rmu.Unlock()

	if !p.rch.send(b, p.rev, p.done) {
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
		if !p.wch.send(b[n:], p.wev, p.done) {
			return n, io.ErrClosedPipe
		}
		r := p.wch.wait()
		n = len(b) - len(r)
	}
	return n, nil
}

func (p *pipe) poll(eventType wasi.EventType) event {
	switch eventType {
	case wasi.FDReadEvent:
		return p.rev
	case wasi.FDWriteEvent:
		return p.wev
	default:
		return nil
	}
}
