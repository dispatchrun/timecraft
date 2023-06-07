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

type Message interface {
	Link() (src, dst net.Addr)

	Pair() int64

	Time() time.Time

	fmt.Formatter

	json.Marshaler

	yaml.Marshaler
}

type ConnProtocol interface {
	Name() string

	CanRead([]byte) bool

	NewClient(fd wasi.FD, addr, peer net.Addr) Conn

	NewServer(fd wasi.FD, addr, peer net.Addr) Conn
}

type Conn interface {
	RecvMessage(now time.Time, data []byte, eof bool) (Message, int, error)

	SendMessage(now time.Time, data []byte, eof bool) (Message, int, error)
}

type MessageReader struct {
	Events stream.Reader[Event]
	Protos []ConnProtocol

	conns  map[wasi.FD]*connection
	offset int
	msgs   []Message
	events [100]Event
}

func (r *MessageReader) Read(msgs []Message) (n int, err error) {
	if r.conns == nil {
		r.conns = make(map[wasi.FD]*connection)
	}

	for {
		if r.offset < len(r.msgs) {
			n = copy(msgs, r.msgs[r.offset:])
			if r.offset += n; r.offset == len(r.msgs) {
				r.offset, r.msgs = 0, r.msgs[:0]
			}
			return n, nil
		}

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
				c := r.conns[event.FD]
				if c == nil {
					continue
				}
				if c.rEOF {
					continue
				}
				c.recv.write(event.Data)
				if !c.hasProto() {
					c.setProto(event.FD, c.recv.bytes(), r.Protos)
				}
				if c.hasProto() {
					r.msgs, _ = c.recvMessages(r.msgs, event.Time, false)
				}

			case Send:
				c := r.conns[event.FD]
				if c == nil {
					continue
				}
				if c.wEOF {
					continue
				}
				c.send.write(event.Data)
				if !c.hasProto() {
					c.setProto(event.FD, c.send.bytes(), r.Protos)
				}
				if c.hasProto() {
					r.msgs, _ = c.sendMessages(r.msgs, event.Time, false)
				}

			case Shutdown:
				c := r.conns[event.FD]
				if c == nil {
					continue
				}
				shutdown := event.Type & EventFlagMask
				if (shutdown & ShutRD) != 0 {
					c.rEOF = true
				}
				if (shutdown & ShutWR) != 0 {
					c.wEOF = true
				}
				if c.rEOF && c.wEOF {
					delete(r.conns, event.FD)
				}
				if !c.hasProto() {
					c.setProto(event.FD, c.send.bytes(), r.Protos)
					c.setProto(event.FD, c.recv.bytes(), r.Protos)
				}
				if c.hasProto() {
					if c.wEOF {
						r.msgs, _ = c.sendMessages(r.msgs, event.Time, true)
					}
					if c.rEOF {
						r.msgs, _ = c.recvMessages(r.msgs, event.Time, true)
					}
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

func (b *buffer) read(n int) []byte {
	data := b.buffer[b.offset : b.offset+n]
	b.offset += n
	return data
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
	recv buffer
	send buffer
	side direction
	rEOF bool
	wEOF bool
}

func (c *connection) hasProto() bool {
	return c.conn != nil
}

func (c *connection) setProto(fd wasi.FD, data []byte, protos []ConnProtocol) {
	for _, proto := range protos {
		if proto.CanRead(data) {
			if c.side == server {
				c.conn = proto.NewServer(fd, c.addr, c.peer)
			} else {
				c.conn = proto.NewClient(fd, c.addr, c.peer)
			}
			break
		}
	}
}

func (c *connection) recvMessages(msgs []Message, now time.Time, eof bool) ([]Message, error) {
	for {
		msg, size, err := c.conn.RecvMessage(now, c.recv.bytes(), eof)
		if err != nil {
			// TODO: how should we handle errors parsing the message?
			return msgs, err
		}
		c.recv.discard(size)
		if msg == nil {
			return msgs, nil
		}
		msgs = append(msgs, msg)
	}
}

func (c *connection) sendMessages(msgs []Message, now time.Time, eof bool) ([]Message, error) {
	for {
		msg, size, err := c.conn.SendMessage(now, c.send.bytes(), eof)
		if err != nil {
			// TODO: how should we handle errors parsing the message?
			return msgs, err
		}
		c.send.discard(size)
		if msg == nil {
			return msgs, nil
		}
		msgs = append(msgs, msg)
	}
}

type direction bool

const (
	server direction = false
	client direction = true
)
