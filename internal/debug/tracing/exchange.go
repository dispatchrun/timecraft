package tracing

import (
	"fmt"
	"io"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"golang.org/x/exp/slices"
)

type Request struct {
	Span time.Duration
	Err  error
	msg  ConnMessage
}

type Response struct {
	Time time.Duration
	Span time.Duration
	Err  error
	msg  ConnMessage
}

type Exchange struct {
	Link Link
	Time time.Time
	Req  Request
	Res  Response
}

func (e Exchange) Format(w fmt.State, v rune) {
	fmt.Fprintf(w, "%s %s %s > %s",
		formatTime(e.Time.Add(e.Res.Time)),
		e.Req.msg.Conn().Protocol().Name(),
		socketAddressString(e.Link.Src),
		socketAddressString(e.Link.Dst))

	if w.Flag('+') {
		fmt.Fprintf(w, "\n")
		e.Req.msg.Format(w, v)
		e.Res.msg.Format(w, v)
	} else {
		fmt.Fprintf(w, ": ")
		e.Req.msg.Format(w, v)

		fmt.Fprintf(w, " => ")
		e.Res.msg.Format(w, v)
	}
}

func (e Exchange) MarshalJSON() ([]byte, error) {
	return []byte(`{}`), nil
}

func (e Exchange) MarshalYAML() (any, error) {
	return nil, nil
}

type ExchangeReader struct {
	Messages stream.Reader[Message]

	inflight  map[int64]Message
	messages  []Message
	exchanges []Exchange
	offset    int
}

func (r *ExchangeReader) Read(exchanges []Exchange) (n int, err error) {
	if r.inflight == nil {
		r.inflight = make(map[int64]Message)
	}
	if len(r.messages) == 0 {
		r.messages = make([]Message, 1000)
	}

	for {
		if r.offset < len(r.exchanges) {
			n = copy(exchanges, r.exchanges[r.offset:])
			if r.offset += n; r.offset == len(r.exchanges) {
				r.offset, r.exchanges = 0, r.exchanges[:0]
			}
			return n, nil
		}

		numMessages, err := stream.ReadFull(r.Messages, r.messages)
		if numMessages == 0 {
			if len(r.inflight) > 0 {
				// The stream was interrupted before receiving a response for
				// some of the inflight requests; generate exchanges with the
				// response error set to io.ErrUnexpectedEOF so we don't mask
				// them from the application.
				for _, req := range r.inflight {
					r.exchanges = append(r.exchanges, Exchange{
						Link: req.Link,
						Time: req.Time,
						Req: Request{
							Span: req.Span,
							Err:  req.Err,
							msg:  req.msg,
						},
						Res: Response{
							Err: io.ErrUnexpectedEOF,
						},
					})
				}
				for id := range r.inflight {
					delete(r.inflight, id)
				}
				sortExchanges(r.exchanges)
				continue
			}
			switch err {
			case nil:
				err = io.ErrNoProgress
			case io.ErrUnexpectedEOF:
				err = io.EOF
			}
			return 0, err
		}

		for i := range r.messages[:numMessages] {
			m := &r.messages[i]

			req, ok := r.inflight[m.id]
			if !ok {
				r.inflight[m.id] = *m
			} else {
				delete(r.inflight, m.id)
				r.exchanges = append(r.exchanges, Exchange{
					Link: req.Link,
					Time: req.Time,
					Req: Request{
						Span: req.Span,
						Err:  req.Err,
						msg:  req.msg,
					},
					Res: Response{
						Time: m.Time.Sub(req.Time),
						Span: m.Span,
						Err:  m.Err,
						msg:  m.msg,
					},
				})
			}
		}

		sortExchanges(r.exchanges)
	}
}

func sortExchanges(exchanges []Exchange) {
	slices.SortFunc(exchanges, func(e1, e2 Exchange) bool {
		t1 := e1.Time.Add(e1.Res.Time)
		t2 := e2.Time.Add(e2.Res.Time)
		return t1.Before(t2)
	})
}
