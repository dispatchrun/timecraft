package sandbox

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stealthrocket/wasi-go"
	"golang.org/x/sys/unix"
)

// system contains the platform-specific state and implementation of the sandbox
// System type.
type system struct {
	pollfds []unix.PollFd

	mutex sync.Mutex
	wake  [2]*os.File
	shut  atomic.Bool
}

type timeout struct {
	duration time.Duration
	subindex int
}

func (s *System) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	if len(subscriptions) == 0 || len(events) < len(subscriptions) {
		return 0, wasi.EINVAL
	}
	r, _, err := s.init()
	if err != nil {
		return 0, wasi.MakeErrno(err)
	}
	s.pollfds = append(s.pollfds[:0], unix.PollFd{
		Fd:     int32(r.Fd()),
		Events: unix.POLLIN | unix.POLLHUP,
	})

	var unixEpoch, now time.Time
	if s.time != nil {
		unixEpoch, now = time.Unix(0, 0), s.time()
	}
	timeout := timeout{duration: -1, subindex: -1}
	setTimeout := func(i int, d time.Duration) {
		if d < 0 {
			d = 0
		}
		if timeout.duration < 0 || d < timeout.duration {
			timeout.subindex = i
			timeout.duration = d
		}
	}

	events = events[:len(subscriptions)]
	for i := range events {
		events[i] = wasi.Event{}
	}
	numEvents := 0

	for i, sub := range subscriptions {
		var pollEvent int16 = unix.POLLPRI | unix.POLLIN | unix.POLLHUP
		switch sub.EventType {
		case wasi.FDWriteEvent:
			pollEvent = unix.POLLOUT
			fallthrough

		case wasi.FDReadEvent:
			fd := sub.GetFDReadWrite().FD
			f, _, errno := s.files.LookupFD(fd, wasi.PollFDReadWriteRight)
			if errno != wasi.ESUCCESS {
				events[i] = makePollError(sub, errno)
				numEvents++
				continue
			}
			s.pollfds = append(s.pollfds, unix.PollFd{
				Fd:     int32(f.Fd()),
				Events: pollEvent,
			})

		case wasi.ClockEvent:
			clock := sub.GetClock()

			var epoch time.Time
			switch clock.ID {
			case wasi.Realtime:
				epoch = unixEpoch
			case wasi.Monotonic:
				epoch = s.epoch
			}
			if epoch.IsZero() {
				events[i] = makePollError(sub, wasi.ENOTSUP)
				numEvents++
				continue
			}
			duration := time.Duration(clock.Timeout)
			if clock.Precision > 0 {
				duration += time.Duration(clock.Precision)
				duration -= 1
			}
			if (clock.Flags & wasi.Abstime) != 0 {
				deadline := epoch.Add(duration)
				setTimeout(i, deadline.Sub(now))
			} else {
				setTimeout(i, duration)
			}
		}
	}

	// We set the timeout to zero when we already produced events due to
	// invalid subscriptions; this is useful to still make progress on I/O
	// completion.
	var deadline time.Time
	if numEvents > 0 {
		timeout.duration = 0
	}
	if timeout.duration > 0 {
		deadline = time.Now().Add(timeout.duration)
	}

	// This loops until either the deadline is reached or at least one event is
	// reported.
	for {
		var timeoutMillis int
		switch {
		case timeout.duration == 0:
			timeoutMillis = 0
		case timeout.duration < 0:
			timeoutMillis = -1
		case !deadline.IsZero():
			timeoutMillis = int(time.Until(deadline).Round(time.Millisecond).Milliseconds())
		}

		n, err := unix.Poll(s.pollfds, timeoutMillis)
		if err != nil && err != unix.EINTR {
			return 0, wasi.MakeErrno(err)
		}

		// poll(2) may cause spurious wake up, so we verify that the system
		// has indeed been shutdown instead of relying on reading the events
		// reported on the first pollfd.
		if s.shut.Load() {
			// If the wake fd was notified it means the system was shut down,
			// we report this by cancelling all subscriptions.
			//
			// Technically we might be erasing events that had already gathered
			// errors in the first loop prior to the call to unix.Poll; this is
			// not a concern since at this time the program would likely be
			// terminating and should not be bothered with handling other
			// errors.
			for i := range subscriptions {
				events[i] = makePollError(subscriptions[i], wasi.ECANCELED)
			}
			return len(subscriptions), wasi.ESUCCESS
		}

		if timeout.subindex >= 0 && deadline.Before(time.Now()) {
			events[timeout.subindex] = makePollEvent(subscriptions[timeout.subindex])
		}

		j := 1
		for i, sub := range subscriptions {
			if events[i].EventType != 0 {
				continue
			}
			switch sub.EventType {
			case wasi.FDReadEvent, wasi.FDWriteEvent:
				pf := &s.pollfds[j]
				j++
				if pf.Revents == 0 {
					continue
				}
				// Linux never reports POLLHUP for disconnected sockets,
				// so there is no reliable mechanism to set wasi.Hanghup.
				// We optimize for portability here and just report that
				// the file descriptor is ready for reading or writing,
				// and let the application deal with the conditions it
				// sees from the following calles to read/write/etc...
				events[i] = makePollEvent(sub)
			}
		}

		// A 1:1 correspondance between the subscription and events arrays is
		// used to track the completion of events, including the completion of
		// invalid subscriptions, clock events, and I/O notifications coming
		// from poll(2).
		//
		// We use zero as the marker on events for subscriptions that have not
		// been fulfilled, but because the zero event type is used to represent
		// clock subscriptions, we mark completed events with the event type+1.
		//
		// The event type is finally restored to its correct value in the loop
		// below when we pack all completed events at the front of the output
		// buffer.
		n = 0

		for _, e := range events {
			if e.EventType != 0 {
				e.EventType--
				events[n] = e
				n++
			}
		}

		if n > 0 {
			return n, wasi.ESUCCESS
		}
	}
}

func makePollEvent(sub wasi.Subscription) wasi.Event {
	return wasi.Event{
		UserData:  sub.UserData,
		EventType: sub.EventType + 1,
	}
}

func makePollError(sub wasi.Subscription, errno wasi.Errno) wasi.Event {
	return wasi.Event{
		UserData:  sub.UserData,
		EventType: sub.EventType + 1,
		Errno:     errno,
	}
}

// Shutdown may be called asynchronously to cancel all blocking operations on
// the system, causing calls such as PollOneOff to unblock and return an
// error indicating that the system is shutting down.
func (s *System) Shutdown(ctx context.Context) error {
	_, w, err := s.init()
	if err != nil {
		if err == context.Canceled {
			err = nil // already shutdown
		}
		return err
	}
	s.shut.Store(true)
	return w.Close()
}

func (s *System) init() (*os.File, *os.File, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.wake[0] == nil {
		if s.shut.Load() {
			return nil, nil, context.Canceled
		}
		r, w, err := os.Pipe()
		if err != nil {
			return nil, nil, err
		}
		s.wake[0] = r
		s.wake[1] = w
	}

	return s.wake[0], s.wake[1], nil
}

func (s *System) close() {
	s.shut.Store(true)
	s.mutex.Lock()
	r := s.wake[0]
	w := s.wake[1]
	s.wake[0] = nil
	s.wake[1] = nil
	s.mutex.Unlock()

	if r != nil {
		r.Close()
	}
	if w != nil {
		w.Close()
	}
}
