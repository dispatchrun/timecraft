package sandbox

import (
	"context"
	"io"
	"io/fs"
	"sync"
	"time"

	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
	"golang.org/x/exp/slices"
)

type Option func(*System)

func Args(args ...string) Option {
	args = slices.Clone(args)
	return func(s *System) { s.args = args }
}

func Env(env ...string) Option {
	env = slices.Clone(env)
	return func(s *System) { s.env = env }
}

func Time(time func() time.Time) Option {
	epoch := time()
	return func(s *System) { s.epoch, s.time = epoch, time }
}

func Rand(rand io.Reader) Option {
	return func(s *System) { s.rand = rand }
}

func FS(fsys fs.FS) Option {
	return func(s *System) { s.fsys = fsys }
}

type System struct {
	args  []string
	env   []string
	epoch time.Time
	time  func() time.Time
	rand  io.Reader
	fsys  fs.FS
	wasi.FileTable[File]
	lock   *sync.Mutex
	stdin  *pipe
	stdout *pipe
	stderr *pipe
	root   wasi.FD
	poll   chan struct{}
}

func New(opts ...Option) *System {
	lock := new(sync.Mutex)

	s := &System{
		lock:   lock,
		stdin:  newPipe(lock),
		stdout: newPipe(lock),
		stderr: newPipe(lock),
		poll:   make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.Preopen(s.stdin, "/dev/stdin", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDWriteRight,
	})
	s.Preopen(s.stdout, "/dev/stdout", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDReadRight,
	})
	s.Preopen(s.stderr, "/dev/stderr", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDReadRight,
	})

	if s.fsys != nil {
		f, err := openFile(s.fsys, ".")
		if err != nil {
			panic(err)
		}
		s.root = s.Preopen(f, "/", wasi.FDStat{
			FileType:         wasi.DirectoryType,
			RightsBase:       wasi.DirectoryRights,
			RightsInheriting: wasi.DirectoryRights | wasi.FileRights,
		})
	}
	return s
}

func (s *System) Stdin() io.WriteCloser {
	return s.stdin
}

func (s *System) Stdout() io.ReadCloser {
	return s.stdout
}

func (s *System) Stderr() io.ReadCloser {
	return s.stderr
}

func (s *System) FS() fs.FS {
	if s.fsys == nil {
		return nil
	}
	// TODO: if we have a use case for it, we might want to pass the context
	// as argument to the method so we can propagate it to the method calls.
	return wasi.FS(context.TODO(), s, s.root)
}

func (s *System) ArgsSizesGet(ctx context.Context) (argCount, stringBytes int, errno wasi.Errno) {
	argCount, stringBytes = wasi.SizesGet(s.args)
	return
}

func (s *System) ArgsGet(ctx context.Context) ([]string, wasi.Errno) {
	return s.args, wasi.ESUCCESS
}

func (s *System) EnvironSizesGet(ctx context.Context) (envCount, stringBytes int, errno wasi.Errno) {
	envCount, stringBytes = wasi.SizesGet(s.env)
	return
}

func (s *System) EnvironGet(ctx context.Context) ([]string, wasi.Errno) {
	return s.env, wasi.ESUCCESS
}

func (s *System) ClockResGet(ctx context.Context, id wasi.ClockID) (wasi.Timestamp, wasi.Errno) {
	switch id {
	case wasi.Realtime:
		return wasi.Timestamp(1), wasi.ESUCCESS
	case wasi.Monotonic:
		return wasi.Timestamp(1), wasi.ESUCCESS
	case wasi.ProcessCPUTimeID, wasi.ThreadCPUTimeID:
		return 0, wasi.ENOTSUP
	default:
		return 0, wasi.EINVAL
	}
}

func (s *System) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	if s.time == nil {
		return 0, wasi.ENOSYS
	}
	now := s.time()
	switch id {
	case wasi.Realtime:
		return wasi.Timestamp(now.UnixNano()), wasi.ESUCCESS
	case wasi.Monotonic:
		return wasi.Timestamp(now.Sub(s.epoch)), wasi.ESUCCESS
	case wasi.ProcessCPUTimeID, wasi.ThreadCPUTimeID:
		return 0, wasi.ENOTSUP
	default:
		return 0, wasi.EINVAL
	}
}

func (s *System) ProcExit(ctx context.Context, code wasi.ExitCode) wasi.Errno {
	panic(sys.NewExitError(uint32(code)))
}

func (s *System) ProcRaise(ctx context.Context, signal wasi.Signal) wasi.Errno {
	panic(sys.NewExitError(127 + uint32(signal)))
}

func (s *System) SchedYield(ctx context.Context) wasi.Errno {
	return wasi.ESUCCESS
}

func (s *System) RandomGet(ctx context.Context, b []byte) wasi.Errno {
	if s.rand == nil {
		return wasi.ENOSYS
	}
	if _, err := io.ReadFull(s.rand, b); err != nil {
		return wasi.EIO
	}
	return wasi.ESUCCESS
}

func (s *System) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	return -1, nil, nil, wasi.ENOSYS
}

func (s *System) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return 0, 0, wasi.ENOSYS
}

func (s *System) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOSYS
}

func (s *System) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	return wasi.ENOSYS
}

func (s *System) SockOpen(ctx context.Context, pf wasi.ProtocolFamily, socketType wasi.SocketType, protocol wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	return -1, wasi.ENOSYS
}

func (s *System) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOSYS
}

func (s *System) SockConnect(ctx context.Context, fd wasi.FD, peer wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOSYS
}

func (s *System) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	return wasi.ENOSYS
}

func (s *System) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOSYS
}

func (s *System) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return 0, 0, nil, wasi.ENOSYS
}

func (s *System) SockGetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return nil, wasi.ENOSYS
}

func (s *System) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return wasi.ENOSYS
}

func (s *System) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOSYS
}

func (s *System) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOSYS
}

func (s *System) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	return 0, wasi.ENOSYS
}

type timeout struct {
	duration time.Duration
	subindex int
}

func (s *System) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	numEvents, timeout, errno := s.pollOneOffScatter(subscriptions, events)
	if errno != wasi.ESUCCESS {
		return numEvents, errno
	}

	if timeout.duration != 0 && numEvents == 0 {
		var deadline <-chan time.Time
		if timeout.duration > 0 {
			t := time.NewTimer(timeout.duration)
			defer t.Stop()
			deadline = t.C
		}
		select {
		case <-s.poll:
		case <-deadline:
			events[numEvents] = wasi.Event{
				UserData:  subscriptions[timeout.subindex].UserData,
				EventType: subscriptions[timeout.subindex].EventType,
			}
			numEvents++
		case <-ctx.Done():
			panic(ctx.Err())
		}
	}

	numEvents += s.pollOneOffGather(subscriptions, events[numEvents:])
	// Clear the event in case it was set after ctx.Done() or deadline
	// triggered.
	select {
	case <-s.poll:
	default:
	}
	return numEvents, wasi.ESUCCESS
}

func (s *System) pollOneOffScatter(subscriptions []wasi.Subscription, events []wasi.Event) (numEvents int, timeout timeout, errno wasi.Errno) {
	if len(subscriptions) == 0 || len(events) < len(subscriptions) {
		return numEvents, timeout, wasi.EINVAL
	}

	timeout.duration = -1
	var unixEpoch = time.Unix(0, 0).UTC()
	var now time.Time
	if s.time != nil {
		now = s.time()
	}

	setTimeout := func(i int, d time.Duration) {
		if d < 0 {
			d = 0
		}
		if timeout.duration < 0 || d < timeout.duration {
			timeout.subindex = i
			timeout.duration = d
		}
	}

	reportError := func(sub wasi.Subscription, errno wasi.Errno) {
		events[numEvents] = wasi.Event{
			UserData:  sub.UserData,
			EventType: sub.EventType,
			Errno:     errno,
		}
		numEvents++
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	for i, sub := range subscriptions {
		switch sub.EventType {
		case wasi.ClockEvent:
			clock := sub.GetClock()

			var epoch time.Time
			switch clock.ID {
			case wasi.Realtime:
				epoch = unixEpoch
			case wasi.Monotonic:
				epoch = s.epoch
			default:
				reportError(sub, wasi.ENOSYS)
				continue
			}

			if (clock.Flags & wasi.Abstime) != 0 {
				deadline := epoch.Add(time.Duration(clock.Timeout + clock.Precision))
				setTimeout(i, deadline.Sub(now))
			} else {
				setTimeout(i, time.Duration(clock.Timeout+clock.Precision))
			}

		case wasi.FDReadEvent, wasi.FDWriteEvent:
			// TODO: check read/write rights
			f, _, errno := s.LookupFD(sub.GetFDReadWrite().FD, 0)
			if errno != wasi.ESUCCESS {
				reportError(sub, errno)
				continue
			}
			if f.Poll(sub.EventType) {
				events[numEvents] = makeFDEvent(sub)
				numEvents++
				continue
			}
			f.Hook(sub.EventType, s.poll)

		default:
			reportError(sub, wasi.ENOTSUP)
		}
	}

	return numEvents, timeout, wasi.ESUCCESS
}

func (s *System) pollOneOffGather(subscriptions []wasi.Subscription, events []wasi.Event) (numEvents int) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, sub := range subscriptions {
		switch sub.EventType {
		case wasi.FDReadEvent, wasi.FDWriteEvent:
			f, _, _ := s.LookupFD(sub.GetFDReadWrite().FD, 0)
			if f == nil {
				continue
			}
			if !f.Poll(sub.EventType) {
				continue
			}
			events[numEvents] = makeFDEvent(sub)
			numEvents++
		}
	}

	return numEvents
}

func makeFDEvent(sub wasi.Subscription) wasi.Event {
	return wasi.Event{
		UserData:  sub.UserData,
		EventType: sub.EventType,
		FDReadWrite: wasi.EventFDReadWrite{
			NBytes: 1, // we don't know how many bytes are available but it's at least one
		},
	}
}

var (
	_ wasi.System = (*System)(nil)
)
