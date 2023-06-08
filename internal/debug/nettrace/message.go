package nettrace

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/wasi-go"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

type Message interface {
	Link() (src, dst net.Addr)

	Pair() int64

	Time() time.Time

	Span() time.Duration

	fmt.Formatter

	json.Marshaler

	yaml.Marshaler
}

type ConnProtocol interface {
	Name() string

	CanHandle(data []byte) bool

	NewClient(fd wasi.FD, addr, peer net.Addr) Conn

	NewServer(fd wasi.FD, addr, peer net.Addr) Conn
}

type Conn interface {
	Observe(*Event)

	Next() Message

	Done() bool
}

type MessageReader struct {
	Events stream.Reader[Event]
	Protos []ConnProtocol

	conns  map[wasi.FD]Conn
	buffer []byte
	events []Event
	msgs   []Message
	offset int
}

func (r *MessageReader) Read(msgs []Message) (n int, err error) {
	if r.conns == nil {
		r.conns = make(map[wasi.FD]Conn)
	}
	if cap(r.events) == 0 {
		r.events = make([]Event, 1000)
	}

	for {
		if r.offset < len(r.msgs) {
			n = copy(msgs, r.msgs[r.offset:])
			if r.offset += n; r.offset == len(r.msgs) {
				r.offset, r.msgs = 0, r.msgs[:0]
			}
			return n, nil
		}

		numEvents, err := r.Events.Read(r.events)
		if numEvents == 0 {
			if err == nil {
				err = io.ErrNoProgress
			}
			return 0, err
		}

		for i := range r.events[:numEvents] {
			e := &r.events[i]

			switch e.Type.Type() {
			case Accept:
				r.conns[e.FD] = &pendingConn{
					newConn: ConnProtocol.NewServer,
				}
				continue
			case Connect:
				r.conns[e.FD] = &pendingConn{
					newConn: ConnProtocol.NewClient,
				}
				continue
			}

			c := r.conns[e.FD]
			switch e.Type.Type() {
			case Receive:
				if pc, _ := c.(*pendingConn); pc != nil {
					c = r.newConn(pc, e)
				}
			case Send:
				if pc, _ := c.(*pendingConn); pc != nil {
					c = r.newConn(pc, e)
				}
			}
			if c == nil {
				continue
			}

			c.Observe(e)
			for {
				if msg := c.Next(); msg != nil {
					r.msgs = append(r.msgs, msg)
				} else if c.Done() {
					delete(r.conns, e.FD)
					break
				} else {
					break
				}
			}
		}
	}
}

func (r *MessageReader) newConn(pc *pendingConn, e *Event) Conn {
	data := r.buffer[:0]
	defer func() { r.buffer = data }()

	if len(pc.events) > 0 {
		data = appendEventData(data, pc.events, e.Type.Type())
	}
	for _, iov := range e.Data {
		data = append(data, iov...)
	}

	proto := findConnProtocol(r.Protos, data)
	if proto == nil {
		pc.events = append(pc.events, e.clone())
		return nil
	}

	conn := pc.newConn(proto, e.FD, e.Addr, e.Peer)
	for i := range pc.events {
		conn.Observe(&pc.events[i])
	}
	conn.Observe(e)
	r.conns[e.FD] = conn
	return conn
}

func appendEventData(buf []byte, events []Event, eventType EventType) []byte {
	for i := range events {
		e := &events[i]
		if e.Type.Type() == eventType {
			for _, iov := range e.Data {
				buf = append(buf, iov...)
			}
		}
	}
	return buf
}

func findConnProtocol(protos []ConnProtocol, data []byte) ConnProtocol {
	for _, proto := range protos {
		if proto.CanHandle(data) {
			return proto
		}
	}
	return nil
}

type timeslice struct {
	time time.Time
	size int
}

type buffer struct {
	times []timeslice
	bytes []byte
}

func (b *buffer) slice(size int) (start, end time.Time, data []byte) {
	start = b.times[0].time
	end = start

	offset := 0
	for i, ts := range b.times {
		if offset += ts.size; offset >= size {
			b.times[i].size -= offset - size
			b.times = b.times[:copy(b.times, b.times[i:])]
			break
		}
		end = ts.time
	}

	data = slices.Clone(b.bytes[:size])
	b.bytes = b.bytes[:copy(b.bytes, b.bytes[size:])]
	return
}

func (b *buffer) write(now time.Time, iovs []Bytes) {
	length := len(b.bytes)

	for _, iov := range iovs {
		b.bytes = append(b.bytes, iov...)
	}

	b.times = append(b.times, timeslice{
		time: now,
		size: len(b.bytes) - length,
	})
}

type pendingConn struct {
	events  []Event
	newConn func(ConnProtocol, wasi.FD, net.Addr, net.Addr) Conn
}

func (*pendingConn) Observe(*Event) {}
func (*pendingConn) Next() Message  { return nil }
func (*pendingConn) Done() bool     { return false }
