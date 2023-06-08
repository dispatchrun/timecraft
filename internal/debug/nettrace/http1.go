package nettrace

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/wasi-go"
)

func HTTP1() ConnProtocol { return http1Protocol{} }

type http1Protocol struct{}

func (http1Protocol) Name() string { return "HTTP" }

func (http1Protocol) CanHandle(data []byte) bool {
	i := bytes.Index(data, []byte("\r\n"))
	if i < 0 {
		return false
	}
	switch line := data[:i]; {
	case bytes.HasPrefix(line, []byte("HTTP/1.1")):
		return true
	case bytes.HasSuffix(line, []byte("HTTP/1.1")):
		return true
	case bytes.HasPrefix(line, []byte("HTTP/1.0")):
		return true
	case bytes.HasSuffix(line, []byte("HTTP/1.0")):
		return true
	}
	return false
}

func (http1Protocol) NewClient(fd wasi.FD, addr, peer net.Addr) Conn {
	conn := &http1Conn{
		addr1: addr,
		addr2: peer,
		reqID: int64(fd) << 32,
		resID: int64(fd) << 32,
	}
	conn.req = &conn.send
	conn.res = &conn.recv
	return conn
}

func (http1Protocol) NewServer(fd wasi.FD, addr, peer net.Addr) Conn {
	conn := &http1Conn{
		addr1: peer,
		addr2: addr,
		reqID: int64(fd) << 32,
		resID: int64(fd) << 32,
	}
	conn.req = &conn.recv
	conn.res = &conn.send
	return conn
}

type http1Conn struct {
	addr1 net.Addr
	addr2 net.Addr
	flag  uint
	recv  buffer
	send  buffer
	req   *buffer
	res   *buffer
	reqID int64
	resID int64
}

func (c *http1Conn) Done() bool {
	const shutdown = ShutRD | ShutWR
	return (c.flag & shutdown) == shutdown
}

func (c *http1Conn) Observe(e *Event) {
	// TODO: capture errors
	switch e.Type.Type() {
	case Receive:
		c.recv.write(e.Time, e.Data)
	case Send:
		c.send.write(e.Time, e.Data)
	case Shutdown:
		c.flag |= e.Type.Flag()
	}
}

func (c *http1Conn) Next() Message {
	if msg := c.nextRequest(); msg != nil {
		return msg
	}
	if msg := c.nextResponse(); msg != nil {
		return msg
	}
	return nil
}

func (c *http1Conn) nextRequest() Message {
	n, err := http1ReadMessage(c.req.bytes, c.Done())
	if err != nil {
		return nil
	}
	if n == 0 {
		return nil
	}
	start, end, data := c.req.slice(n)
	c.reqID++
	return &http1Request{
		http1Message: http1Message{
			pair: c.reqID,
			src:  c.addr1,
			dst:  c.addr2,
			time: start,
			span: end.Sub(start),
			data: data,
		},
	}
}

func (c *http1Conn) nextResponse() Message {
	n, err := http1ReadMessage(c.res.bytes, c.Done())
	if err != nil {
		return nil
	}
	if n == 0 {
		return nil
	}
	start, end, data := c.res.slice(n)
	c.resID++
	return &http1Response{
		http1Message: http1Message{
			pair: c.resID,
			src:  c.addr2,
			dst:  c.addr1,
			time: start,
			span: end.Sub(start),
			data: data,
		},
	}
}

var (
	http1HeaderSeparator      = []byte(": ")
	http1RequestFormatPrefix  = []byte("> ")
	http1ResponseFormatPrefix = []byte("< ")
	newLine                   = []byte("\n")
)

type http1Message struct {
	src  net.Addr
	dst  net.Addr
	pair int64
	time time.Time
	span time.Duration
	data []byte
}

func (msg *http1Message) Link() (net.Addr, net.Addr) { return msg.src, msg.dst }

func (msg *http1Message) Pair() int64 { return msg.pair }

func (msg *http1Message) Time() time.Time { return msg.time }

func (msg *http1Message) Span() time.Duration { return msg.span }

func (msg *http1Message) format(state fmt.State, verb rune, prefix []byte) {
	fmt.Fprintf(state, "%s HTTP %s > %s",
		formatTime(msg.time),
		socketAddressString(msg.src),
		socketAddressString(msg.dst))

	if state.Flag('+') {
		fmt.Fprintf(state, "\n")
	} else {
		fmt.Fprintf(state, ": ")
	}

	header, body, _ := http1SplitMessage(msg.data)
	status, header := http1SplitLine(header)
	status = bytes.TrimSpace(status)

	if state.Flag('+') {
		w := textprint.NewPrefixWriter(state, prefix)
		w.Write(status)
		w.Write(newLine)

		http1HeaderRange(header, func(name, value []byte) bool {
			w.Write(name)
			w.Write(http1HeaderSeparator)
			w.Write(value)
			w.Write(newLine)
			return true
		})

		w.Write(newLine)
		switch verb {
		case 'x':
			hexdump := hex.Dumper(w)
			_, _ = hexdump.Write(body)
			hexdump.Close()

		default:
			if utf8.Valid(body) {
				w.Write(body)
			} else {
				fmt.Fprintf(w, "(binary content)")
			}
		}
	} else {
		status = bytes.TrimPrefix(status, []byte("HTTP/1.0 "))
		status = bytes.TrimPrefix(status, []byte("HTTP/1.1 "))
		status = bytes.TrimSuffix(status, []byte(" HTTP/1.0"))
		status = bytes.TrimSuffix(status, []byte(" HTTP/1.1"))
		state.Write(status)
	}
}

type http1Request struct {
	http1Message
}

func (req *http1Request) Format(w fmt.State, v rune) {
	req.format(w, v, http1RequestFormatPrefix)
}

func (req *http1Request) MarshalJSON() ([]byte, error) {
	return []byte(`{}`), nil
}

func (req *http1Request) MarshalYAML() (any, error) {
	return nil, nil
}

type http1Response struct {
	http1Message
}

func (res *http1Response) Format(w fmt.State, v rune) {
	res.format(w, v, http1ResponseFormatPrefix)
}

func (res *http1Response) MarshalJSON() ([]byte, error) {
	return []byte(`{}`), nil
}

func (res *http1Response) MarshalYAML() (any, error) {
	return nil, nil
}

func http1ReadMessage(msg []byte, eof bool) (n int, err error) {
	header, _, ok := http1SplitMessage(msg)
	if !ok {
		return 0, nil
	}
	messageLength := len(header)
	contentLength := -1

	statusLine, header := http1SplitLine(header)
	http1HeaderRange(header, func(name, value []byte) bool {
		// TODO: Transfer-Encoding
		if !bytes.EqualFold(name, []byte("Content-Length")) {
			return true
		}
		v, parseErr := strconv.ParseInt(string(value), 10, 32)
		if parseErr != nil {
			err = fmt.Errorf("malformed http content-length header: %w", parseErr)
		} else if v < 0 {
			err = fmt.Errorf("malformed http content-length header: %d", v)
		} else {
			contentLength = int(v)
		}
		return false
	})
	if err != nil {
		return 0, err
	}

	if contentLength >= 0 {
		return messageLength + contentLength, nil
	}

	method, statusCode, _ := http1SplitStatusLine(statusLine)
	if bytes.HasPrefix(method, []byte("HTTP")) {
		switch string(statusCode) {
		case "204":
			return messageLength, nil
		}
	} else {
		switch string(method) {
		case "HEAD", "GET":
			return messageLength, nil
		}
	}

	if !eof {
		// At this stage we know that there must be a body, but we don't have
		// enough bytes to decode it all.
		return 0, nil
	}

	// We exhausted all options, this indicates that the body extends until the
	// connection has reached EOF, all remaining bytes are part of it.
	return len(msg), nil
}

func http1HeaderRange(header []byte, do func(name, value []byte) bool) {
	for len(header) > 0 {
		var line []byte
		line, header = http1SplitLine(header)
		name, value := http1SplitHeaderLine(line)
		if len(name) == 0 {
			break
		}
		if !do(name, value) {
			break
		}
	}
}

func http1SplitStatusLine(b []byte) (part1, part2, part3 []byte) {
	part1, b, _ = split(b, []byte(" "))
	part2, part3, _ = split(b, []byte(" "))
	part1 = bytes.TrimSpace(part1)
	part2 = bytes.TrimSpace(part2)
	part3 = bytes.TrimSpace(part3)
	return
}

func http1SplitLine(b []byte) (line, next []byte) {
	line, next, _ = split(b, []byte("\r\n"))
	return
}

func http1SplitMessage(b []byte) (header, body []byte, ok bool) {
	header, body, ok = split(b, []byte("\r\n\r\n"))
	return
}

func http1SplitHeaderLine(b []byte) (name, value []byte) {
	name, value, _ = split(b, []byte(":"))
	name = bytes.TrimSpace(name)
	name = bytes.TrimSuffix(name, []byte(":"))
	value = bytes.TrimSpace(value)
	return
}

func split(b, sep []byte) (head, tail []byte, ok bool) {
	i := bytes.Index(b, sep)
	if i < 0 {
		return b, nil, false
	}
	i += len(sep)
	return b[:i], b[i:], true
}
