package nettrace

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/wasi-go"
	"gopkg.in/yaml.v3"
)

type Exchange interface {
	Link() (src, dst net.Addr)

	Time() time.Time

	Duration() time.Duration

	Request() Message

	Response() Message
}

type Message interface {
	fmt.Formatter
	json.Marshaler
	yaml.Marshaler
}

type ConnProtocol interface {
	Name() string

	CanRead([]byte) bool

	NewClient(addr, peer net.Addr) Conn

	NewServer(addr, peer net.Addr) Conn
}

type Conn interface {
	Recv(now time.Time, data []byte)

	Send(now time.Time, data []byte)

	Shut(now time.Time)

	Next(version uint64) Exchange
}

type ExchangeReader struct {
	Events stream.Reader[Event]
	Protos []ConnProtocol

	conns     map[wasi.FD]*connection
	exchanges []Exchange
	offset    int
	version   uint64
	events    [100]Event
}

func (r *ExchangeReader) Read(exchanges []Exchange) (n int, err error) {
	if len(r.Protos) == 0 {
		return 0, io.EOF
	}
	if len(exchanges) == 0 {
		return 0, nil
	}
	if r.conns == nil {
		r.conns = make(map[wasi.FD]*connection)
	}

	for {
		if r.offset < len(r.exchanges) {
			n = copy(exchanges, r.exchanges[r.offset:])
			r.exchanges = r.exchanges[:copy(r.exchanges, r.exchanges[r.offset+n:])]
			r.offset = 0
			return n, nil
		}

		r.version++
		numEvents, err := r.Events.Read(r.events[:])
		if numEvents == 0 {
			if err == nil {
				err = io.ErrNoProgress
			}
			return 0, err
		}

		for _, event := range r.events[:numEvents] {
			switch event.Type & EventTypeMask {
			case Accept:
				r.conns[event.FD] = &connection{
					addr: event.Addr,
					peer: event.Peer,
					side: server,
				}

			case Connect:
				r.conns[event.FD] = &connection{
					addr: event.Addr,
					peer: event.Peer,
					side: client,
				}

			case Receive:
				if c := r.conns[event.FD]; c != nil {
					c.recv(event.Time, event.Data, r.Protos)
					r.exchanges = c.next(r.exchanges, r.version)
				}

			case Send:
				if c := r.conns[event.FD]; c != nil {
					c.send(event.Time, event.Data, r.Protos)
					r.exchanges = c.next(r.exchanges, r.version)
				}

			case Shutdown:
				if c := r.conns[event.FD]; c != nil {
					c.shut(event.Time, event.Type, r.Protos)
					r.exchanges = c.next(r.exchanges, r.version)
				}
			}
		}
	}
}

type buffer struct {
	buffer []byte
	offset int
}

func (b *buffer) bytes() []byte {
	return b.buffer[b.offset:]
}

func (b *buffer) discard(n int) {
	if b.offset += n; b.offset == len(b.buffer) {
		b.buffer = b.buffer[:0]
		b.offset = 0
	}
}

func (b *buffer) write(iovs []Bytes) {
	if b.offset == len(b.buffer) {
		b.buffer = b.buffer[:0]
		b.offset = 0
	}
	for _, iov := range iovs {
		b.buffer = append(b.buffer, iov...)
	}
}

type connection struct {
	conn Conn
	addr net.Addr
	peer net.Addr
	rbuf []byte
	wbuf []byte
	side direction
	rEOF bool
	wEOF bool
}

type direction bool

const (
	server direction = false
	client direction = true
)

func (c *connection) setProto(now time.Time, data []byte, protos []ConnProtocol) {
	for _, proto := range protos {
		if proto.CanRead(data) {
			if c.side == server {
				c.conn = proto.NewServer(c.addr, c.peer)
			} else {
				c.conn = proto.NewClient(c.addr, c.peer)
			}
			c.conn.Recv(now, c.rbuf)
			c.conn.Send(now, c.wbuf)
			break
		}
	}
}

func (c *connection) shut(now time.Time, typ EventType, protos []ConnProtocol) {
	if (typ & ShutRD) != 0 {
		c.rEOF = true
	}
	if (typ & ShutWR) != 0 {
		c.wEOF = true
	}
	if c.rEOF && c.wEOF && c.conn != nil {
		c.conn.Shut(now)
	}
}

func (c *connection) recv(now time.Time, iovs []Bytes, protos []ConnProtocol) {
	if len(iovs) == 0 {
		return
	}

	if c.conn == nil {
		for i, iov := range iovs {
			c.rbuf = append(c.rbuf, iov...)
			c.setProto(now, c.rbuf, protos)
			if c.conn != nil {
				iovs = iovs[i+1:]
				break
			}
		}
	}

	if c.conn != nil {
		for _, iov := range iovs {
			c.conn.Recv(now, iov)
		}
	}
}

func (c *connection) send(now time.Time, iovs []Bytes, protos []ConnProtocol) {
	if len(iovs) == 0 {
		return
	}

	if c.conn == nil {
		for i, iov := range iovs {
			c.wbuf = append(c.wbuf, iov...)
			c.setProto(now, c.wbuf, protos)
			if c.conn != nil {
				iovs = iovs[i+1:]
				break
			}
		}
	}

	if c.conn != nil {
		for _, iov := range iovs {
			c.conn.Send(now, iov)
		}
	}
}

func (c *connection) next(exchanges []Exchange, version uint64) []Exchange {
	if c.conn != nil {
		for {
			e := c.conn.Next(version)
			if e == nil {
				break
			}
			exchanges = append(exchanges, e)
		}
	}
	return exchanges
}
