package tracing

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/wasi-go"
)

// HTTP1 is the implementation of the HTTP/1 protocol.
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
	recv  http1Parser
	send  http1Parser
	req   *http1Parser
	res   *http1Parser
	reqID int64
	resID int64
}

func (c *http1Conn) Protocol() ConnProtocol {
	return http1Protocol{}
}

func (c *http1Conn) Done() bool {
	const shutdown = ShutRD | ShutWR
	return (c.flag & shutdown) == shutdown
}

func (c *http1Conn) Observe(e *Event) {
	// TODO: capture errors
	switch e.Type.Type() {
	case Receive:
		c.recv.write(e.Time, e.Data, false)
	case Send:
		c.send.write(e.Time, e.Data, false)
	case Shutdown:
		c.flag |= e.Type.Flag()
		c.recv.write(e.Time, e.Data, (c.flag&ShutRD) != 0)
		c.send.write(e.Time, e.Data, (c.flag&ShutWR) != 0)
	}
}

func (c *http1Conn) Next(msg *Message) bool {
	return c.nextRequest(msg) || c.nextResponse(msg)
}

func (c *http1Conn) nextRequest(msg *Message) bool {
	start, end, data, err, ok := c.req.read()
	if !ok {
		return false
	}
	c.reqID++
	msg.Link = Link{Src: c.addr1, Dst: c.addr2}
	msg.Time = start
	msg.Span = end.Sub(start)
	msg.Err = err
	msg.id = c.reqID
	msg.msg = &http1Request{
		http1Message: http1Message{
			conn: c,
			data: data,
		},
	}
	return true
}

func (c *http1Conn) nextResponse(msg *Message) bool {
	start, end, data, err, ok := c.res.read()
	if !ok {
		if c.Done() && c.resID < c.reqID {
			// TODO: set the start time to the last event start time
			err = io.ErrUnexpectedEOF
		} else {
			return false
		}
	}
	c.resID++
	msg.Link = Link{Src: c.addr2, Dst: c.addr1}
	msg.Time = start
	msg.Span = end.Sub(start)
	msg.Err = err
	msg.id = c.resID
	msg.msg = &http1Response{
		http1Message: http1Message{
			conn: c,
			data: data,
		},
	}
	return true
}

var (
	http1HeaderSeparator      = []byte(": ")
	http1RequestFormatPrefix  = []byte("> ")
	http1ResponseFormatPrefix = []byte("< ")
	newLine                   = []byte("\n")
)

type http1Message struct {
	conn *http1Conn
	data []byte
}

func (msg *http1Message) Conn() Conn { return msg.conn }

func (msg *http1Message) parse() (startLine, header, body []byte) {
	header, body, _ = http1SplitMessage(msg.data)
	startLine, header = http1SplitLine(header)
	return http1TrimCRLFSuffix(startLine), header, body
}

func (msg *http1Message) format(state fmt.State, verb rune, prefix []byte) {
	startLine, header, body := msg.parse()

	if state.Flag('+') {
		w := textprint.NewPrefixWriter(state, prefix)
		write := func(b []byte) { _, _ = w.Write(b) }
		write(startLine)
		write(newLine)

		unparsed := http1HeaderRange(header, func(name, value []byte) bool {
			write(name)
			write(http1HeaderSeparator)
			write(http1TrimCRLFSuffix(value))
			write(newLine)
			return true
		})

		write(unparsed)
		write(newLine)
		switch verb {
		case 'x':
			hexdump := hex.Dumper(state)
			_, _ = hexdump.Write(body)
			hexdump.Close()

		default:
			if utf8.Valid(body) {
				state.Write(body)
			} else {
				fmt.Fprintf(state, "(binary content)")
			}
		}
	} else {
		startLine = bytes.TrimPrefix(startLine, []byte("HTTP/1.0 "))
		startLine = bytes.TrimPrefix(startLine, []byte("HTTP/1.1 "))
		startLine = bytes.TrimSuffix(startLine, []byte(" HTTP/1.0"))
		startLine = bytes.TrimSuffix(startLine, []byte(" HTTP/1.1"))
		state.Write(startLine)
	}
}

type http1Request struct {
	http1Message
}

func (req *http1Request) Format(w fmt.State, v rune) {
	req.format(w, v, http1RequestFormatPrefix)
}

func (req *http1Request) Marshal() any {
	startLine, header, body := req.parse()
	method, path, proto := http1SplitStartLine(startLine)
	return &httpRequest{
		Proto:  string(proto),
		Method: string(method),
		Path:   string(path),
		Header: http1FormatHeader(header),
		Body:   Bytes(body),
	}
}

type http1Response struct {
	http1Message
}

func (res *http1Response) Format(w fmt.State, v rune) {
	res.format(w, v, http1ResponseFormatPrefix)
}

func (res *http1Response) Marshal() any {
	startLine, header, body := res.parse()
	proto, statusCodeString, statusText := http1SplitStartLine(startLine)
	statusCode, _ := strconv.Atoi(string(statusCodeString))
	return &httpResponse{
		Proto:      string(proto),
		StatusCode: statusCode,
		StatusText: string(statusText),
		Header:     http1FormatHeader(header),
		Body:       Bytes(body),
	}
}

func http1FormatHeader(b []byte) map[string]string {
	header := make(map[string]string)
	http1HeaderRange(b, func(name, value []byte) bool {
		k := textproto.CanonicalMIMEHeaderKey(string(name))
		v, exist := header[k]
		if exist {
			v += ", " + string(value)
		} else {
			v = string(value)
		}
		header[k] = v
		return true
	})
	return header
}

type http1Parser struct {
	buffer        buffer
	headerLength  int
	contentLength int
	err           error
}

func (p *http1Parser) read() (start, end time.Time, data []byte, err error, ok bool) {
	if p.headerLength == 0 || p.contentLength < 0 {
		return
	}
	start, end, data = p.buffer.slice(p.headerLength + p.contentLength)
	err = p.err
	p.headerLength = 0
	p.contentLength = 0
	p.err = nil
	return start, end, data, err, true
}

func (p *http1Parser) write(now time.Time, data []Bytes, eof bool) {
	p.buffer.write(now, data)

	if p.headerLength == 0 {
		// TODO: set a limit to how large a http header may be
		header, _, ok := http1SplitMessage(p.buffer.bytes)
		if !ok {
			return
		}
		p.headerLength = len(header)
		p.contentLength = -1
	}

	if p.contentLength < 0 {
		startLine, header := http1SplitLine(p.buffer.bytes[:p.headerLength])
		http1HeaderRange(header, func(name, value []byte) bool {
			// TODO: Transfer-Encoding
			if !bytes.EqualFold(name, []byte("Content-Length")) {
				return true
			}
			v, parseErr := strconv.ParseInt(string(value), 10, 32)
			if parseErr != nil {
				p.err = fmt.Errorf("malformed http content-length header: %w", parseErr)
			} else if v < 0 {
				p.err = fmt.Errorf("malformed http content-length header: %d", v)
			} else {
				p.contentLength = int(v)
			}
			return false
		})

		if p.contentLength >= 0 {
			return
		}

		method, statusCode, _ := http1SplitStartLine(startLine)
		if bytes.HasPrefix(method, []byte("HTTP")) {
			switch string(statusCode) {
			case "204":
				p.contentLength = 0
				return
			}
		} else {
			switch string(method) {
			case "HEAD", "GET":
				p.contentLength = 0
				return
			}
		}

		if !eof {
			// At this stage we know that there must be a body, but we don't have
			// enough bytes to decode it all.
			return
		}

		// We exhausted all options, this indicates that the body extends until the
		// connection has reached EOF, all remaining bytes are part of it.
		p.contentLength = len(p.buffer.bytes) - p.headerLength
		return
	}
}

func http1ParseFieldName(b []byte) (name, remain []byte) {
	i := bytes.IndexByte(b, ':')
	if i < 0 {
		return nil, b
	}
	return b[:i:i], b[i+1:]
}

func http1ParseFieldValue(b []byte) (value, remain []byte) {
	for len(b) > 0 {
		var lws bool
		lws, b = http1ParseLWS(b)
		if lws {
			continue
		}
		if http1HasCRLFPrefix(b) {
			return value, b
		}
		var v []byte
		v, b = http1ParseFieldContent(b)

		if len(value) != 0 {
			value = append(value, ' ')
			value = append(value, v...)
		} else {
			value = v
		}
	}
	return value, nil
}

func http1ParseFieldContent(b []byte) (content, remain []byte) {
	i := bytes.Index(b, []byte("\r\n")) // simplify, just look for the potential end of header field
	if i < 0 {
		return b, nil
	}
	return b[:i:i], b[i:]
}

func http1ParseCRLF(b []byte) (ok bool, remain []byte) {
	if http1HasCRLFPrefix(b) {
		return true, b[2:]
	}
	return false, b
}

func http1ParseLWS(b []byte) (ok bool, remain []byte) {
	switch {
	case len(b) == 0:
	case http1IsSpace(b[0]):
		return true, b[1:]
	case http1HasCRLFPrefix(b):
		if len(b) > 2 && http1IsSpace(b[2]) {
			return true, b[3:]
		}
	}
	return false, b
}

func http1HeaderRange(header []byte, do func(name, value []byte) bool) []byte {
	// message-header = field-name ":" [ field-value ]
	// field-name     = token
	// field-value    = *( field-content | LWS )
	// field-content  = <the OCTETs making up the field-value
	//                     and consisting of either *TEXT or combinations
	//                     of token, separators, and quoted-string>
	var name, value []byte
	for {
		name, header = http1ParseFieldName(header)
		if len(name) == 0 {
			break
		}
		value, header = http1ParseFieldValue(header)
		if len(value) == 0 {
			break
		}
		if !do(name, value) {
			break
		}
		header = http1TrimCRLFPrefix(header)
	}
	if len(header) > 0 {
		header = http1TrimCRLFPrefix(header)
	}
	return header
}

func http1SplitStartLine(b []byte) (part1, part2, part3 []byte) {
	// start-line   = Request-Line | Status-Line
	// Request-Line = Method SP Request-URI SP HTTP-Version CRLF
	// Status-Line  = HTTP-Version SP Status-Code SP Reason-Phrase CRLF
	i := bytes.IndexByte(b, ' ')
	if i < 0 {
		part1 = b
		return
	}
	part1, b = b[:i], b[i+1:]

	i = bytes.IndexByte(b, ' ')
	if i < 0 {
		part2 = b
		return
	}

	part2, part3 = b[:i], b[i+1:]
	return part1, part2, http1TrimCRLFSuffix(part3)
}

func http1SplitLine(b []byte) (line, next []byte) {
	line, next, _ = split(b, []byte("\r\n"))
	return
}

func http1SplitMessage(b []byte) (header, body []byte, ok bool) {
	header, body, ok = split(b, []byte("\r\n\r\n"))
	return
}

func http1IsSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func http1HasCRLFPrefix(b []byte) bool {
	return bytes.HasPrefix(b, []byte("\r\n"))
}

func http1TrimCRLFPrefix(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte("\r\n"))
}

func http1TrimCRLFSuffix(b []byte) []byte {
	return bytes.TrimSuffix(b, []byte("\r\n"))
}

func split(b, sep []byte) (head, tail []byte, ok bool) {
	i := bytes.Index(b, sep)
	if i < 0 {
		return b, nil, false
	}
	i += len(sep)
	return b[:i], b[i:], true
}
