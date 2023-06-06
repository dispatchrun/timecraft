package nettrace

import (
	"encoding"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
)

type Bytes []byte

func (b Bytes) MarshalYAML() (any, error) {
	return base64.StdEncoding.EncodeToString(b), nil
}

type Protocol uint8

const (
	TCP Protocol = 1
	UDP Protocol = 2
)

func (p Protocol) String() string {
	switch p {
	case TCP:
		return "TCP"
	case UDP:
		return "UDP"
	default:
		return "NOP"
	}
}

func (p Protocol) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *Protocol) UnmarshalText(b []byte) error {
	switch string(b) {
	case "TCP":
		*p = TCP
	case "UDP":
		*p = UDP
	case "NOP", "":
		*p = 0
	default:
		return fmt.Errorf("unsupported network protocol name: %q", b)
	}
	return nil
}

var (
	_ fmt.Stringer             = Protocol(0)
	_ encoding.TextMarshaler   = Protocol(0)
	_ encoding.TextUnmarshaler = (*Protocol)(nil)
)

type EventType uint8

const (
	Accept EventType = iota + 1
	Connect
	Receive
	Send
	Shutdown
	// extra flags associated with the Shut event type
	ShutRD = 1 << 6
	ShutWR = 1 << 7
	// mask to extract the even type or flags independently
	EventTypeMask = 0b00111111
	EventFlagMask = 0b11000000
)

func (t EventType) String() string {
	switch t & EventTypeMask {
	case Accept:
		return "ACCEPT"
	case Connect:
		return "CONN"
	case Receive:
		return "RECV"
	case Send:
		return "SEND"
	case Shutdown:
		switch t & EventFlagMask {
		case ShutRD:
			return "SHUT (r)"
		case ShutWR:
			return "SHUT (w)"
		default:
			return "SHUT"
		}
	default:
		return "NOP"
	}
}

func (t EventType) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *EventType) UnmarshalText(b []byte) error {
	switch string(b) {
	case "ACCEPT":
		*t = Accept
	case "CONN":
		*t = Connect
	case "RECV":
		*t = Receive
	case "SEND":
		*t = Send
	case "SHUT":
		*t = Shutdown | ShutRD | ShutWR
	case "SHUT (r)":
		*t = Shutdown | ShutRD
	case "SHUT (w)":
		*t = Shutdown | ShutWR
	case "NOP", "":
		*t = 0
	default:
		return fmt.Errorf("malformed network event type: %q", b)
	}
	return nil
}

var (
	_ fmt.Stringer             = EventType(0)
	_ encoding.TextMarshaler   = EventType(0)
	_ encoding.TextUnmarshaler = (*EventType)(nil)
)

type Event struct {
	Record int64      `json:"record"          yaml:"record"`
	Time   time.Time  `json:"time"            yaml:"time"`
	Type   EventType  `json:"type"            yaml:"type"`
	Proto  Protocol   `json:"proto"           yaml:"proto"`
	Error  wasi.Errno `json:"error,omitempty" yaml:"error,omitempty"`
	FD     wasi.FD    `json:"fd"              yaml:"fd"`
	Addr   net.Addr   `json:"addr,omitempty"  yaml:"addr,omitempty"`
	Peer   net.Addr   `json:"peer,omitempty"  yaml:"peer,omitempty"`
	Data   []Bytes    `json:"data,omitempty"  yaml:"data,omitempty"`
}

func (e Event) Format(w fmt.State, _ rune) {
	src := e.Addr
	dst := e.Peer

	switch e.Type & EventTypeMask {
	case Accept, Receive:
		src, dst = dst, src
	}

	if w.Flag('+') {
		fmt.Fprintf(w, "[%d] ", e.Record)
	}

	fmt.Fprintf(w, "%s %s %s > %s: %s %s",
		e.Time.In(time.Local).Format("2006/01/02 15:04:05.000000"),
		e.Proto,
		socketAddressString(src),
		socketAddressString(dst),
		e.Type,
		errnoName(e.Error))

	if e.Type == Receive || e.Type == Send {
		fmt.Fprintf(w, " %d", iovecSize(e.Data))
	}

	fmt.Fprintln(w)

	if w.Flag('+') {
		if (e.Type == Receive || e.Type == Send) && iovecSize(e.Data) > 0 {
			fmt.Fprintln(w)
			hexdump := hex.Dumper(w)
			for _, iov := range e.Data {
				_, _ = hexdump.Write(iov)
			}
			hexdump.Close()
			fmt.Fprintln(w)
		}
	}
}

func errnoName(errno wasi.Errno) string {
	if errno == wasi.ESUCCESS {
		return "OK"
	}
	return errno.Name()
}

func iovecSize(iovs []Bytes) (size wasi.Size) {
	for _, iov := range iovs {
		size += wasi.Size(len(iov))
	}
	return size
}

func socketAddressString(addr net.Addr) string {
	if addr == nil {
		return "?"
	}
	return addr.String()
}

func (e *Event) init(offset int64, s *socket, time time.Time, typ EventType, errno wasi.Errno) {
	*e = Event{
		Record: offset,
		Time:   time,
		Type:   typ,
		Proto:  s.proto,
		Error:  errno,
		FD:     s.fd,
		Addr:   s.addr,
		Peer:   s.peer,
		Data:   e.Data[:0],
	}
}

func (e *Event) write(iovs []wasi.IOVec, size wasi.Size) {
	for _, iov := range iovs {
		iovLen := wasi.Size(len(iov))
		if iovLen > size {
			iovLen = size
			iov = iov[:size]
		}
		if iovLen != 0 {
			size -= iovLen
			e.Data = append(e.Data, Bytes(iov))
		}
	}
}

var (
	_ fmt.Formatter = Event{}
)

type EventReader struct {
	Records stream.Reader[timemachine.Record]

	sockets map[wasi.FD]*socket
	records []timemachine.Record
	iovecs  []wasi.IOVec
	codec   wasicall.Codec
}

type socket struct {
	proto Protocol
	fd    wasi.FD
	shut  wasi.SDFlags
	addr  wasi.SocketAddress
	peer  wasi.SocketAddress
}

func (r *EventReader) Read(events []Event) (n int, err error) {
	if cap(r.records) < len(events) {
		r.records = make([]timemachine.Record, len(events))
	} else {
		r.records = r.records[:len(events)]
	}

	if r.sockets == nil {
		r.sockets = make(map[wasi.FD]*socket)
	}

	rn, err := r.Records.Read(r.records)

	for _, record := range r.records[:rn] {
		switch wasicall.SyscallID(record.FunctionID) {
		case wasicall.FDClose:
			fd, errno, err := r.codec.DecodeFDClose(record.FunctionCall)
			if err != nil {
				return n, err
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			delete(r.sockets, fd)
			shutdown := Shutdown
			if (socket.shut & wasi.ShutdownRD) == 0 {
				shutdown |= ShutRD
			}
			if (socket.shut & wasi.ShutdownWR) == 0 {
				shutdown |= ShutWR
			}
			events[n].init(record.Offset, socket, record.Time, shutdown, errno)
			n++

		case wasicall.FDRenumber:
			from, to, errno, err := r.codec.DecodeFDRenumber(record.FunctionCall)
			if err != nil {
				return n, err
			}
			if errno != wasi.ESUCCESS {
				continue
			}
			socket, ok := r.sockets[from]
			if !ok {
				continue
			}
			delete(r.sockets, from)
			r.sockets[to] = socket

		case wasicall.FDRead:
			fd, iovecs, size, errno, err := r.codec.DecodeFDRead(record.FunctionCall, r.iovecs[:0])
			if err != nil {
				return n, err
			}
			r.iovecs = iovecs
			if errno == wasi.EAGAIN {
				errno = wasi.ESUCCESS
			}
			if (errno == wasi.ESUCCESS) && (int32(size) <= 0) {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			events[n].init(record.Offset, socket, record.Time, Receive, errno)
			events[n].write(iovecs, size)
			n++

		case wasicall.FDWrite:
			fd, iovecs, size, errno, err := r.codec.DecodeFDWrite(record.FunctionCall, r.iovecs[:0])
			if err != nil {
				return n, err
			}
			r.iovecs = iovecs
			if errno == wasi.EAGAIN {
				errno = wasi.ESUCCESS
			}
			if (errno == wasi.ESUCCESS) && (int32(size) <= 0) {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			events[n].init(record.Offset, socket, record.Time, Send, errno)
			events[n].write(iovecs, size)
			n++

		case wasicall.SockAccept:
			fd, _, newfd, peer, addr, errno, err := r.codec.DecodeSockAccept(record.FunctionCall)
			if err != nil {
				return n, err
			}
			server, ok := r.sockets[fd]
			if !ok {
				// This condition may occur when using a preopen socket
				// which has not been seen before by any other function.
				server = &socket{proto: TCP, fd: fd, addr: addr}
				r.sockets[fd] = server
			}
			if errno == wasi.EAGAIN {
				continue
			}
			if errno == wasi.ESUCCESS {
				client := &socket{proto: server.proto, fd: newfd, addr: addr, peer: peer}
				r.sockets[newfd] = client
				events[n].init(record.Offset, client, record.Time, Accept, 0)
			} else {
				events[n].init(record.Offset, server, record.Time, Accept, errno)
			}
			n++

		case wasicall.SockRecv:
			fd, iovecs, _, size, _, errno, err := r.codec.DecodeSockRecv(record.FunctionCall, r.iovecs[:0])
			if err != nil {
				return n, err
			}
			r.iovecs = iovecs
			if errno == wasi.EAGAIN {
				errno = wasi.ESUCCESS
			}
			if (errno == wasi.ESUCCESS) && (int32(size) <= 0) {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			events[n].init(record.Offset, socket, record.Time, Receive, errno)
			events[n].write(iovecs, size)
			n++

		case wasicall.SockSend:
			fd, iovecs, _, size, errno, err := r.codec.DecodeSockSend(record.FunctionCall)
			if err != nil {
				return n, err
			}
			r.iovecs = iovecs
			if errno == wasi.EAGAIN {
				errno = wasi.ESUCCESS
			}
			if (errno == wasi.ESUCCESS) && (int32(size) <= 0) {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			events[n].init(record.Offset, socket, record.Time, Send, errno)
			events[n].write(iovecs, size)
			n++

		case wasicall.SockShutdown:
			fd, flags, errno, err := r.codec.DecodeSockShutdown(record.FunctionCall)
			if err != nil {
				return n, err
			}
			if errno != wasi.ESUCCESS {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			if flags &= ^socket.shut; flags == 0 {
				continue
			}
			socket.shut |= flags
			shutdown := Shutdown
			if (flags & wasi.ShutdownRD) != 0 {
				shutdown |= ShutRD
			}
			if (flags & wasi.ShutdownWR) != 0 {
				shutdown |= ShutWR
			}
			events[n].init(record.Offset, socket, record.Time, shutdown, 0)
			n++

		case wasicall.SockOpen:
			_, _, protocol, _, _, fd, errno, err := r.codec.DecodeSockOpen(record.FunctionCall)
			if err != nil {
				return n, err
			}
			if errno != wasi.ESUCCESS {
				continue
			}
			var proto Protocol
			switch protocol {
			case wasi.TCPProtocol:
				proto = TCP
			case wasi.UDPProtocol:
				proto = UDP
			default:
				continue
			}
			socket := &socket{proto: proto, fd: fd}
			r.sockets[fd] = socket

		case wasicall.SockBind:
			fd, _, addr, errno, err := r.codec.DecodeSockBind(record.FunctionCall)
			if err != nil {
				return n, err
			}
			if errno != wasi.ESUCCESS {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			socket.addr = addr

		case wasicall.SockConnect:
			fd, peer, addr, errno, err := r.codec.DecodeSockConnect(record.FunctionCall)
			if err != nil {
				return n, err
			}
			if errno != wasi.ESUCCESS && errno != wasi.EINPROGRESS {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			socket.addr = addr
			socket.peer = peer
			events[n].init(record.Offset, socket, record.Time, Connect, errno)
			n++

		case wasicall.SockRecvFrom:
			fd, iovecs, _, size, _, addr, errno, err := r.codec.DecodeSockRecvFrom(record.FunctionCall, r.iovecs[:0])
			if err != nil {
				return n, err
			}
			r.iovecs = iovecs
			if errno == wasi.EAGAIN {
				errno = wasi.ESUCCESS
			}
			if (errno == wasi.ESUCCESS) && (int32(size) <= 0) {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			events[n].init(record.Offset, socket, record.Time, Receive, errno)
			events[n].write(iovecs, size)
			events[n].Peer = addr
			n++

		case wasicall.SockSendTo:
			fd, iovecs, _, addr, size, errno, err := r.codec.DecodeSockSendTo(record.FunctionCall, r.iovecs[:0])
			if err != nil {
				return n, err
			}
			r.iovecs = iovecs
			if errno == wasi.EAGAIN {
				errno = wasi.ESUCCESS
			}
			if (errno == wasi.ESUCCESS) && (int32(size) <= 0) {
				continue
			}
			socket, ok := r.sockets[fd]
			if !ok {
				continue
			}
			events[n].init(record.Offset, socket, record.Time, Send, errno)
			events[n].write(iovecs, size)
			events[n].Peer = addr
			n++
		}
	}

	return n, err
}

var (
	_ stream.Reader[Event] = (*EventReader)(nil)
)
