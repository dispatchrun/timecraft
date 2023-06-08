package tracing

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/wasi-go"
	"golang.org/x/exp/slices"
)

// Link represents a network connection initiated from a source address to a
// local or remote destination.
type Link struct {
	Src net.Addr
	Dst net.Addr
}

// Message is a representation of a message exchanged between two network peers.
type Message struct {
	Link Link
	Time time.Time
	Span time.Duration
	Err  error
	// The id is used for correlation between requests and responses when using
	// an ExchangeReader.
	id  int64
	msg ConnMessage
}

func (m Message) Format(w fmt.State, v rune) {
	fmt.Fprintf(w, "%s %s %s > %s",
		formatTime(m.Time),
		m.msg.Conn().Protocol().Name(),
		socketAddressString(m.Link.Src),
		socketAddressString(m.Link.Dst))

	if w.Flag('+') {
		fmt.Fprintf(w, "\n")
	} else {
		fmt.Fprintf(w, ": ")
	}

	m.msg.Format(w, v)
}

func (m Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.marshal())
}

func (m Message) MarshalYAML() (any, error) {
	return m.marshal(), nil
}

func (m *Message) marshal() *format.Message {
	return &format.Message{
		Link: format.Link(m.Link),
		Time: m.Time,
		Span: m.Span,
		Err:  errorString(m.Err),
		Msg:  m.msg.Marshal(),
	}
}

func errorString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// ConnProtocol is an interface implemented by types which represent high level
// connection-oriented protocols such as HTTP.
type ConnProtocol interface {
	// Returns the name of the protocol.
	//
	// This value is used when printing protocol messages in a human readable
	// format.
	Name() string

	// Given a data segment found at the beginning of a network connection,
	// determine whether the protocol is able to decode it.
	//
	// This method is used to do automatic detection of the protocol being used
	// when inspecting network events.
	CanHandle(data []byte) bool

	// Constructs a Conn instance intended to decode protocol messages exchanged
	// over a client connection.
	NewClient(fd wasi.FD, addr, peer net.Addr) Conn

	// Constructs a Conn instance intended to decode protocol messages exchanged
	// over a server connection.
	NewServer(fd wasi.FD, addr, peer net.Addr) Conn
}

// ConnMessage represents a message read from a client or server connection.
type ConnMessage interface {
	// Returns the connection that the message was originally decoded from.
	Conn() Conn

	// Marshals the message into a representation intended to be marshaled to
	// structured formats like JSON or YAML.
	Marshal() any

	// Formats the message to a human readable format.
	fmt.Formatter
}

// Conn values represent client and server connections.
//
// The instances are intended to reconstruct the state of a connection from
// observing a sequence of network events which contain data fragments seen
// by the connection.
type Conn interface {
	// Returns the protocol that this Conn value was constructed from.
	Protocol() ConnProtocol

	// Observe is called when a network event is received for this connection.
	//
	// The receiver is expected to update its state by capturing the data
	// fragments carried in the event.
	Observe(*Event)

	// After calls to Observe, the connection may receive a call to Next in
	// order to consume the next message that was formed by the accumulation of
	// data fragements from network events.
	//
	// The method writes to the Message value passed as argument, returning true
	// if a message could be formed, and false otherwise.
	Next(*Message) bool

	// Indicates whether the connection has reached EOF, and has no more
	// messages to read.
	Done() bool
}

// MessageReader is a reader of Message values. Instances of MessageReader
// consume network events from a reader of Event values and reconstruct network
// messages exchanged over network connections.
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
	if len(r.events) == 0 {
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

		numEvents, err := stream.ReadFull(r.Events, r.events)
		if numEvents == 0 {
			switch err {
			case nil:
				err = io.ErrNoProgress
			case io.ErrUnexpectedEOF:
				err = io.EOF
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
				i := len(r.msgs)
				r.msgs = append(r.msgs, Message{})

				if !c.Next(&r.msgs[i]) {
					r.msgs = r.msgs[:i]
					if c.Done() {
						delete(r.conns, e.FD)
					}
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

func (b *buffer) write(now time.Time, iovs []format.Bytes) {
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

func (*pendingConn) Protocol() ConnProtocol { return nil }
func (*pendingConn) Next(*Message) bool     { return false }
func (*pendingConn) Done() bool             { return false }
func (*pendingConn) Observe(*Event)         {}
