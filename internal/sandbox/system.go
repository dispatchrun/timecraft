package sandbox

import (
	"context"
	"io"
	"io/fs"
	"reflect"
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
	wasi.FileTable[anyFile]
	stdin  pipe
	stdout pipe
	stderr pipe
	root   wasi.FD
	subs   []wasi.Subscription
	poll   []reflect.SelectCase
}

func New(opts ...Option) *System {
	s := &System{
		stdin:  makePipe(),
		stdout: makePipe(),
		stderr: makePipe(),
	}

	for _, opt := range opts {
		opt(s)
	}

	s.Preopen(&s.stdin, "/dev/stdin", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDWriteRight,
	})
	s.Preopen(&s.stdout, "/dev/stdout", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDReadRight,
	})
	s.Preopen(&s.stderr, "/dev/stderr", wasi.FDStat{
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
	return nil
}

func (s *System) Stdout() io.ReadCloser {
	return nil
}

func (s *System) Stderr() io.ReadCloser {
	return nil
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
		if s.epoch.IsZero() {
			return 0, wasi.ENOTSUP
		}
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

func (s *System) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	/*
		if len(subscriptions) == 0 || len(events) < len(subscriptions) {
			return 0, wasi.EINVAL
		}

		subs := s.subs[:0]
		poll := s.poll[:0]
		defer func() {
			for i := range poll {
				poll[i] = reflect.SelectCase{}
			}
			s.subs = subs
			s.poll = poll
		}()

		numEvents := 0
		timeout := time.Duration(-1)
		realtime := time.Time{}
		monotonic := time.Time{}

		if s.time != nil {
			realtime = s.time()
			monontonic = realtime.Sub(s.epoch)
		}

		for i, sub := range subscriptions {
			switch sub.EventType {
			case wasi.ClockEvent:
				switch sub.ID {
				case wasi.Realtime:
					if !realtime.IsZero() {

					}
				case wasi.Monotonic:
					if !monotonic.IsZero() {

					}
				}

				events[numEvents] = wasi.Event{
					UserData:  sub.UserData,
					Errno:     wasi.ENOSYS,
					EventType: sub.EventType,
				}
				numEvents++

			case wasi.FDReadEvent, wasi.FDWriteEvent:
				f, _, errno := s.LookupFD(sub.GetFDReadWrite().FD, 0)
				if errno != wasi.ESUCCESS {
					events[numEvents] = wasi.Event{
						UserData:  sub.UserData,
						Errno:     errno,
						EventType: sub.EventType,
					}
					numEvents++
				} else {
					subs = append(subs, sub)
					poll = append(poll, reflect.SelectCase{
						Dir:  reflect.SelectRecv,
						Chan: reflect.ValueOf(f.poll(sub.EventType)),
					})
				}
			}
		}

		if numEvents > 0 {
			return numEvents, wasi.ESUCCESS
		}

		switch {
		case timeout == 0:
			poll = append(poll, reflect.SelectCase{
				Dir: reflect.SelectDefault,
			})
		case timeout > 0:
			t := time.NewTimer(timeout)
			defer t.Stop()
			poll = append(poll, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(t.C),
			})
		}

		poll = append(poll, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		})

		chosen, _, _ := reflect.Select(poll)
		if chosen == len(poll)-1 {
			return 0, wasi.MakeErrno(ctx.Err())
		}

		n := len(poll) - 1
		switch {
		case timeout == 0:
			n--
		case timeout > 0:
		}

		for i, poll := range poll[:len(poll)-1] {
			_, recv := poll.Chan.TryRecv()
			if recv || i == chosen {
				events[numEvents] = wasi.Event{
					UserData:  subs[i].UserData,
					EventType: subs[i].EventType,
					FDReadWrite: wasi.EventFDReadWrite{
						NBytes: 1, // at least one byte is available
					},
				}
				numEvents++
			}
		}
	*/
	return 0, wasi.ENOSYS
}

var (
	_ wasi.System = (*System)(nil)
)
