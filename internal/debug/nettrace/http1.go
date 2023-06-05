package nettrace

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"time"
	"unicode/utf8"
)

func HTTP1() ConnProtocol { return http1Protocol{} }

type http1Protocol struct{}

func (http1Protocol) Name() string {
	return "HTTP"
}

func (http1Protocol) CanRead(msg []byte) bool {
	i := bytes.Index(msg, []byte("\r\n"))
	if i < 0 {
		return false
	}
	switch line := msg[:i]; {
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

func (http1Protocol) NewClient(addr, peer net.Addr) Conn {
	return &http1ClientConn{
		http1Conn: http1Conn{
			addr: addr,
			peer: peer,
		},
	}
}

func (http1Protocol) NewServer(addr, peer net.Addr) Conn {
	return &http1ServerConn{
		http1Conn: http1Conn{
			addr: addr,
			peer: peer,
		},
	}
}

type http1Conn struct {
	addr net.Addr
	peer net.Addr
}

func (c *http1Conn) Protocol() ConnProtocol {
	return http1Protocol{}
}

func (c *http1Conn) LocalAddr() net.Addr {
	return c.addr
}

func (c *http1Conn) RemoteAddr() net.Addr {
	return c.peer
}

type http1ClientConn struct {
	http1Conn
}

func (c *http1ClientConn) RecvMessage(now time.Time, data []byte, eof bool) (Message, int, error) {
	return http1ReadResponse(c, now, data, eof)
}

func (c *http1ClientConn) SendMessage(now time.Time, data []byte, eof bool) (Message, int, error) {
	return http1ReadRequest(c, now, data, eof)
}

type http1ServerConn struct {
	http1Conn
}

func (c *http1ServerConn) RecvMessage(now time.Time, data []byte, eof bool) (Message, int, error) {
	return http1ReadRequest(c, now, data, eof)
}

func (c *http1ServerConn) SendMessage(now time.Time, data []byte, eof bool) (Message, int, error) {
	return http1ReadResponse(c, now, data, eof)
}

var (
	http1HeaderSeparator = []byte(": ")
	newLine              = []byte("\n")
)

type http1Request struct {
	conn Conn
	time time.Time
	data []byte
}

func (req *http1Request) Conn() Conn      { return req.conn }
func (req *http1Request) Time() time.Time { return req.time }
func (req *http1Request) Data() []byte    { return req.data }

func (req *http1Request) Format(w fmt.State, v rune) {
	http1FormatMessage(w, v, req.conn, req.time, req.data)
}

func (req *http1Request) MarshalJSON() ([]byte, error) {
	return []byte(`{}`), nil
}

func (req *http1Request) MarshalYAML() (any, error) {
	return nil, nil
}

type http1Response struct {
	conn Conn
	time time.Time
	data []byte
}

func (res *http1Response) Conn() Conn      { return res.conn }
func (res *http1Response) Time() time.Time { return res.time }
func (res *http1Response) Data() []byte    { return res.data }

func (req *http1Response) Format(w fmt.State, v rune) {
	http1FormatMessage(w, v, req.conn, req.time, req.data)
}

func (req *http1Response) MarshalJSON() ([]byte, error) {
	return []byte(`{}`), nil
}

func (req *http1Response) MarshalYAML() (any, error) {
	return nil, nil
}

func http1FormatMessage(w fmt.State, v rune, conn Conn, time time.Time, data []byte) {
	fmt.Fprintf(w, "%s HTTP %s > %s",
		formatTime(time),
		socketAddressString(conn.LocalAddr()),
		socketAddressString(conn.RemoteAddr()))

	if w.Flag('+') {
		fmt.Fprintf(w, "\n")
	} else {
		fmt.Fprintf(w, ": ")
	}

	header, body, _ := http1SplitMessage(data)
	status, header := http1SplitLine(header)
	status = bytes.TrimSpace(status)
	w.Write(status)

	if w.Flag('+') {
		w.Write(newLine)

		http1HeaderRange(header, func(name, value []byte) bool {
			w.Write(name)
			w.Write(http1HeaderSeparator)
			w.Write(value)
			w.Write(newLine)
			return true
		})

		switch v {
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
	}
}

func http1ReadMessage(msg []byte, eof bool) (n int, err error) {
	header, _, ok := http1SplitMessage(msg)
	if !ok {
		return 0, nil
	}
	messageLength := len(header)
	contentLength := -1

	_, header = http1SplitLine(header)
	http1HeaderRange(header, func(name, value []byte) bool {
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

	if contentLength < 0 {
		if !eof {
			return 0, nil
		}
		return len(msg), nil
	}

	return messageLength + contentLength, nil
}

func http1ReadRequest(conn Conn, now time.Time, msg []byte, eof bool) (Message, int, error) {
	messageLength, err := http1ReadMessage(msg, eof)
	if err != nil {
		return nil, 0, err
	}
	if messageLength == 0 {
		return nil, 0, nil
	}
	req := &http1Request{
		conn: conn,
		time: now,
		data: msg[:messageLength],
	}
	return req, messageLength, nil
}

func http1ReadResponse(conn Conn, now time.Time, msg []byte, eof bool) (Message, int, error) {
	messageLength, err := http1ReadMessage(msg, eof)
	if err != nil {
		return nil, 0, err
	}
	if messageLength == 0 {
		return nil, 0, nil
	}
	res := &http1Response{
		conn: conn,
		time: now,
		data: msg[:messageLength],
	}
	return res, messageLength, nil
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
