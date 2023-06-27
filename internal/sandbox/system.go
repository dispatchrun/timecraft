package sandbox

import (
	"context"
	"io"
	"io/fs"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
	"golang.org/x/exp/slices"
)

// Option represents configuration options that can be set when instantiating a
// System.
type Option func(*System)

// Args configures the list of arguments passed to the guest module.
func Args(args ...string) Option {
	args = slices.Clone(args)
	return func(s *System) { s.args = args }
}

// Environ configures the list of environment variables exposed to the guest
// module.
func Environ(environ ...string) Option {
	environ = slices.Clone(environ)
	return func(s *System) { s.env = environ }
}

// Time configures the function used by the guest module to get the current
// time.
//
// If not set, the guest does not have access to the current time.
func Time(time func() time.Time) Option {
	return func(s *System) { s.time = time }
}

// Rand configures the random number generator exposed to the guest module.
//
// If not set, the guest cannot generate random numbers.
func Rand(rand io.Reader) Option {
	return func(s *System) { s.rand = rand }
}

// Mount configures a mount point to expose a file system to the guest module.
//
// If no endpoints are set, the guest does not have a file system.
func Mount(path string, fsys FS) Option {
	return func(s *System) {
		s.mounts = append(s.mounts, mountPoint{
			path: path,
			fsys: fsys,
		})
	}
}

// Socket configures a unix socket to be exposed to the guest module.
func Socket(name string) Option {
	return func(s *System) { s.unix.name = name }
}

// Dial configures a dial function used to establish network connections from
// the guest module.
//
// If not set, the guest module cannot open outbound connections.
func Dial(dial func(context.Context, string, string) (net.Conn, error)) Option {
	return func(s *System) {
		s.ipv4.dialFunc = dial
		s.ipv6.dialFunc = dial
		s.unix.dialFunc = dial
	}
}

// Listen configures the function used to create listeners accepting connections
// from the host network and routing them to a listening socket on the guest.
//
// The creation of listeners is driven by the guest, when it opens a listening
// socket, the listen function is invoked with the port number that the socket
// is bound to in order to create a bridge between the host and guest network.
//
// If not set, the guest module cannot accept inbound connections frrom the host
// network.
func Listen(listen func(context.Context, string, string) (net.Listener, error)) Option {
	return func(s *System) {
		s.ipv4.listenFunc = listen
		s.ipv6.listenFunc = listen
		s.unix.listenFunc = listen
	}
}

// ListenPacket configures the function used to create datagram sockets on the
// host network.
//
// If not set, the guest module cannot open host datagram sockets.
func ListenPacket(listenPacket func(context.Context, string, string) (net.PacketConn, error)) Option {
	return func(s *System) {
		s.ipv4.listenPacketFunc = listenPacket
		s.ipv6.listenPacketFunc = listenPacket
		s.unix.listenPacketFunc = listenPacket
	}
}

// IPv4Network configures the network used by the sandbox IPv4 network.
//
// Default to "127.0.0.1/8"
func IPv4Network(ipnet netip.Prefix) Option {
	return func(s *System) { s.ipv4.ipnet = ipnet }
}

// IPv6Network configures the network used by the sandbox IPv6 network.
//
// Default to "::1/128"
func IPv6Network(ipnet netip.Prefix) Option {
	return func(s *System) { s.ipv6.ipnet = ipnet }
}

// Resolver configures the name resolver used when the guest attempts to
// lookup addresses.
//
// Default to disabling name resolution.
func Resolver(rslv ServiceResolver) Option {
	return func(s *System) { s.rslv = rslv }
}

// System is an implementation of the wasi.System interface which sandboxes all
// interactions of the guest module with the world.
type System struct {
	args  []string
	env   []string
	epoch time.Time
	time  func() time.Time
	rand  io.Reader
	wasi.FileTable[File]
	poll   chan struct{}
	lock   *sync.Mutex
	stdin  *pipe
	stdout *pipe
	stderr *pipe
	root   wasi.FD
	ipv4   ipnet[ipv4]
	ipv6   ipnet[ipv6]
	unix   unixnet
	rslv   ServiceResolver
	mounts []mountPoint
}

type mountPoint struct {
	path string
	fsys FS
}

// New creates a new System instance, applying the list of options passed as
// arguments.
func New(opts ...Option) *System {
	lock := new(sync.Mutex)
	dial := func(context.Context, string, string) (net.Conn, error) {
		return nil, syscall.ECONNREFUSED
	}
	listen := func(context.Context, string, string) (net.Listener, error) {
		return nil, syscall.EOPNOTSUPP
	}
	listenPacket := func(context.Context, string, string) (net.PacketConn, error) {
		return nil, syscall.EOPNOTSUPP
	}

	s := &System{
		lock:   lock,
		stdin:  newPipe(lock),
		stdout: newPipe(lock),
		stderr: newPipe(lock),
		poll:   make(chan struct{}, 1),
		root:   ^wasi.FD(0),
		rslv:   defaultResolver{},

		ipv4: ipnet[ipv4]{
			ipnet:            netip.PrefixFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 8),
			dialFunc:         dial,
			listenFunc:       listen,
			listenPacketFunc: listenPacket,
		},

		ipv6: ipnet[ipv6]{
			ipnet:            netip.PrefixFrom(netip.AddrFrom16([16]byte{15: 1}), 128),
			dialFunc:         dial,
			listenFunc:       listen,
			listenPacketFunc: listenPacket,
		},

		unix: unixnet{
			dialFunc:         dial,
			listenFunc:       listen,
			listenPacketFunc: listenPacket,
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.Preopen(input{s.stdin}, "/dev/stdin", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDWriteRight,
	})
	s.Preopen(output{s.stdout}, "/dev/stdout", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDReadRight,
	})
	s.Preopen(output{s.stderr}, "/dev/stderr", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDReadRight,
	})

	for _, mount := range s.mounts {
		f, errno := mount.fsys.PathOpen(context.Background(),
			wasi.LookupFlags(0),
			".",
			wasi.OpenDirectory,
			wasi.DirectoryRights,
			wasi.DirectoryRights|wasi.FileRights,
			wasi.FDFlags(0),
		)
		if errno != wasi.ESUCCESS {
			panic(&fs.PathError{Op: "open", Path: mount.path, Err: errno.Syscall()})
		}
		fd := s.Preopen(f, mount.path, wasi.FDStat{
			FileType:         wasi.DirectoryType,
			RightsBase:       wasi.DirectoryRights,
			RightsInheriting: wasi.DirectoryRights | wasi.FileRights,
		})
		if s.root == ^wasi.FD(0) {
			// TODO: this is a bit of a hack intended to pass the fstest test
			// suite, ideally we should have a mechanism to create a unified
			// view of all mount points when converting to a fs.FS.
			s.root = fd
		}
	}

	if s.time != nil {
		s.epoch = s.time()
	}
	return s
}

// Stdin returns a writer to the standard input of the guest module.
func (s *System) Stdin() io.WriteCloser { return inputWriteCloser{s.stdin} }

// Stdout returns a writer to the standard output of the guest module.
func (s *System) Stdout() io.ReadCloser { return outputReadCloser{s.stdout} }

// Stderr returns a writer to the standard output of the guest module.
func (s *System) Stderr() io.ReadCloser { return outputReadCloser{s.stderr} }

// FS returns a fs.FS exposing the file system mounted to the guest module.
func (s *System) FS() fs.FS {
	if s.root == ^wasi.FD(0) {
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
	sock, stat, errno := s.LookupSocketFD(fd, wasi.SockAcceptRight)
	if errno != wasi.ESUCCESS {
		return ^wasi.FD(0), nil, nil, errno
	}
	conn, errno := sock.SockAccept(ctx, flags)
	if errno != wasi.ESUCCESS {
		return ^wasi.FD(0), nil, nil, errno
	}
	addr, errno := conn.SockLocalAddress(ctx)
	if errno != wasi.ESUCCESS {
		_ = conn.FDClose(ctx)
		return ^wasi.FD(0), nil, nil, wasi.ECONNREFUSED
	}
	peer, errno := conn.SockRemoteAddress(ctx)
	if errno != wasi.ESUCCESS {
		_ = conn.FDClose(ctx)
		return ^wasi.FD(0), nil, nil, wasi.ECONNREFUSED
	}
	newFD := s.Register(conn, wasi.FDStat{
		Flags:      flags,
		FileType:   stat.FileType,
		RightsBase: stat.RightsInheriting,
	})
	return newFD, peer, addr, wasi.ESUCCESS
}

func (s *System) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return 0, 0, errno
	}
	return sock.SockRecv(ctx, iovecs, flags)
}

func (s *System) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return 0, errno
	}
	return sock.SockSend(ctx, iovecs, flags)
}

func (s *System) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	if (flags & ^(wasi.ShutdownRD | wasi.ShutdownWR)) != 0 {
		return wasi.EINVAL
	}
	sock, _, errno := s.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return sock.SockShutdown(ctx, flags)
}

func (s *System) SockOpen(ctx context.Context, pf wasi.ProtocolFamily, st wasi.SocketType, proto wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	const none = ^wasi.FD(0)

	switch proto {
	case wasi.IPProtocol:
	case wasi.TCPProtocol:
	case wasi.UDPProtocol:
	default:
		return none, wasi.EPROTOTYPE
	}

	if st == wasi.AnySocket {
		switch proto {
		case wasi.TCPProtocol:
			st = wasi.StreamSocket
		case wasi.UDPProtocol:
			st = wasi.DatagramSocket
		default:
			return none, wasi.EPROTOTYPE
		}
	}

	var support bool
	switch pf {
	case wasi.InetFamily:
		support = s.ipv4.supports(protocol(proto))
	case wasi.Inet6Family:
		support = s.ipv6.supports(protocol(proto))
	case wasi.UnixFamily:
		support = s.unix.supports(protocol(proto))
	}
	if !support {
		return none, wasi.EPROTONOSUPPORT
	}
	if !socktype(st).supports(protocol(proto)) {
		return none, wasi.EPROTONOSUPPORT
	}

	if proto == wasi.IPProtocol {
		switch pf {
		case wasi.InetFamily, wasi.Inet6Family:
			if st == wasi.StreamSocket {
				proto = wasi.TCPProtocol
			} else {
				proto = wasi.UDPProtocol
			}
		}
	}

	var socket File
	switch pf {
	case wasi.InetFamily:
		socket = newSocket[ipv4](&s.ipv4, socktype(st), protocol(proto), s.lock, s.poll)
	case wasi.Inet6Family:
		socket = newSocket[ipv6](&s.ipv6, socktype(st), protocol(proto), s.lock, s.poll)
	case wasi.UnixFamily:
		socket = newSocket[unix](&s.unix, socktype(st), protocol(proto), s.lock, s.poll)
	default:
		return none, wasi.EAFNOSUPPORT
	}

	var fileType wasi.FileType
	switch st {
	case wasi.StreamSocket:
		fileType = wasi.SocketStreamType
	case wasi.DatagramSocket:
		fileType = wasi.SocketDGramType
	}

	newFD := s.Register(socket, wasi.FDStat{
		FileType:         fileType,
		RightsBase:       rightsBase,
		RightsInheriting: rightsInheriting,
	})
	return newFD, wasi.ESUCCESS
}

func (s *System) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, wasi.SockAcceptRight)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	if errno := sock.SockBind(ctx, addr); errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockLocalAddress(ctx)
}

func (s *System) SockConnect(ctx context.Context, fd wasi.FD, peer wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	errno = sock.SockConnect(ctx, peer)
	switch errno {
	case wasi.ESUCCESS, wasi.EINPROGRESS:
		addr, _ := sock.SockLocalAddress(ctx)
		return addr, errno
	default:
		return nil, errno
	}
}

func (s *System) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	sock, _, errno := s.LookupSocketFD(fd, wasi.SockAcceptRight)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return sock.SockListen(ctx, backlog)
}

func (s *System) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return 0, errno
	}
	return sock.SockSendTo(ctx, iovecs, flags, addr)
}

func (s *System) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, wasi.FDReadRight)
	if errno != wasi.ESUCCESS {
		return 0, 0, nil, errno
	}
	return sock.SockRecvFrom(ctx, iovecs, flags)
}

func (s *System) SockGetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockGetOpt(ctx, level, option)
}

func (s *System) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	sock, _, errno := s.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return sock.SockSetOpt(ctx, level, option, value)
}

func (s *System) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockLocalAddress(ctx)
}

func (s *System) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockRemoteAddress(ctx)
}

func (s *System) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	if len(results) == 0 {
		return 0, wasi.EINVAL
	}
	// TODO: support AI_ADDRCONFIG, AI_CANONNAME, AI_V4MAPPED, AI_V4MAPPED_CFG, AI_ALL

	var network string
	f, p, t := hints.Family, hints.Protocol, hints.SocketType
	switch {
	case t == wasi.StreamSocket && p != wasi.UDPProtocol:
		switch f {
		case wasi.UnspecifiedFamily:
			network = "tcp"
		case wasi.InetFamily:
			network = "tcp4"
		case wasi.Inet6Family:
			network = "tcp6"
		default:
			return 0, wasi.ENOTSUP // EAI_FAMILY
		}
	case t == wasi.DatagramSocket && p != wasi.TCPProtocol:
		switch f {
		case wasi.UnspecifiedFamily:
			network = "udp"
		case wasi.InetFamily:
			network = "udp4"
		case wasi.Inet6Family:
			network = "udp6"
		default:
			return 0, wasi.ENOTSUP // EAI_FAMILY
		}
	case t == wasi.AnySocket:
		switch f {
		case wasi.UnspecifiedFamily:
			network = "ip"
		case wasi.InetFamily:
			network = "ip4"
		case wasi.Inet6Family:
			network = "ip6"
		default:
			return 0, wasi.ENOTSUP // EAI_FAMILY
		}
	default:
		return 0, wasi.ENOTSUP // EAI_SOCKTYPE / EAI_PROTOCOL
	}

	var port int
	var err error
	if hints.Flags.Has(wasi.NumericService) {
		port, err = strconv.Atoi(service)
	} else {
		port, err = s.rslv.LookupPort(ctx, network, service)
	}
	if err != nil || port < 0 || port > 65535 {
		return 0, wasi.EINVAL // EAI_NONAME / EAI_SERVICE
	}

	var ip net.IP
	if hints.Flags.Has(wasi.NumericHost) {
		ip = net.ParseIP(name)
		if ip == nil {
			return 0, wasi.EINVAL
		}
	} else if name == "" {
		if !hints.Flags.Has(wasi.Passive) {
			return 0, wasi.EINVAL
		}
		if hints.Family == wasi.Inet6Family {
			ip = net.IPv6zero
		} else {
			ip = net.IPv4zero
		}
	}

	makeAddressInfo := func(ip net.IP, port int) wasi.AddressInfo {
		addrInfo := wasi.AddressInfo{
			Flags:      hints.Flags,
			SocketType: hints.SocketType,
			Protocol:   hints.Protocol,
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			inet4Addr := &wasi.Inet4Address{Port: port}
			copy(inet4Addr.Addr[:], ipv4)
			addrInfo.Family = wasi.InetFamily
			addrInfo.Address = inet4Addr
		} else {
			inet6Addr := &wasi.Inet6Address{Port: port}
			copy(inet6Addr.Addr[:], ip)
			addrInfo.Family = wasi.Inet6Family
			addrInfo.Address = inet6Addr
		}
		return addrInfo
	}

	if ip != nil {
		results[0] = makeAddressInfo(ip, port)
		return 1, wasi.ESUCCESS
	}

	// LookupIP requires the network to be one of "ip", "ip4", or "ip6".
	switch network {
	case "tcp", "udp":
		network = "ip"
	case "tcp4", "udp4":
		network = "ip4"
	case "tcp6", "udp6":
		network = "ip6"
	}

	ips, err := s.rslv.LookupIP(ctx, network, name)
	if err != nil {
		return 0, wasi.ECANCELED // TODO: better errors on name resolution failure
	}

	addrs4 := make([]wasi.AddressInfo, 0, 8)
	addrs6 := make([]wasi.AddressInfo, 0, 8)

	for _, ip := range ips {
		if ip.To4() != nil {
			addrs4 = append(addrs4, makeAddressInfo(ip, port))
		} else {
			addrs6 = append(addrs6, makeAddressInfo(ip, port))
		}
	}

	n := copy(results[0:], addrs4)
	n += copy(results[n:], addrs6)
	return n, wasi.ESUCCESS
}

type timeout struct {
	duration time.Duration
	subindex int
}

func (s *System) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	if len(subscriptions) == 0 || len(events) < len(subscriptions) {
		return 0, wasi.EINVAL
	}
	events = events[:len(subscriptions)]
	for i := range events {
		events[i] = wasi.Event{}
	}

	numEvents, timeout, errno := s.pollOneOffScatter(subscriptions, events)
	if errno != wasi.ESUCCESS {
		return numEvents, errno
	}
	if numEvents == 0 && timeout.duration != 0 {
		s.pollOneOffWait(ctx, subscriptions, events, timeout)
	}
	s.pollOneOffGather(subscriptions, events)
	// Clear the event in case it was set after ctx.Done() or deadline
	// triggered.
	select {
	case <-s.poll:
	default:
	}

	n := 0
	for _, e := range events {
		if e.EventType != 0 {
			e.EventType--
			events[n] = e
			n++
		}
	}
	return n, wasi.ESUCCESS
}

func (s *System) pollOneOffWait(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event, timeout timeout) {
	var deadline <-chan time.Time
	if timeout.duration > 0 {
		t := time.NewTimer(timeout.duration)
		defer t.Stop()
		deadline = t.C
	}
	select {
	case <-s.poll:
	case <-deadline:
		events[timeout.subindex] = makePollEvent(subscriptions[timeout.subindex])
	case <-ctx.Done():
		panic(ctx.Err())
	}
}

func (s *System) pollOneOffScatter(subscriptions []wasi.Subscription, events []wasi.Event) (numEvents int, timeout timeout, errno wasi.Errno) {
	_ = events[:len(subscriptions)]

	timeout.duration = -1
	var unixEpoch, now time.Time
	if s.time != nil {
		unixEpoch, now = time.Unix(0, 0), s.time()
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
			}
			if epoch.IsZero() {
				events[i] = makePollError(sub, wasi.ENOTSUP)
				numEvents++
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
				events[i] = makePollError(sub, errno)
				numEvents++
			} else if f.FDPoll(sub.EventType, s.poll) {
				events[i] = makePollEvent(sub)
				numEvents++
			}

		default:
			events[i] = makePollError(sub, wasi.ENOTSUP)
			numEvents++
		}
	}

	if timeout.duration == 0 {
		events[timeout.subindex] = makePollEvent(subscriptions[timeout.subindex])
		numEvents++
	}

	return numEvents, timeout, wasi.ESUCCESS
}

func (s *System) pollOneOffGather(subscriptions []wasi.Subscription, events []wasi.Event) {
	_ = events[:len(subscriptions)]

	s.lock.Lock()
	defer s.lock.Unlock()

	for i, sub := range subscriptions {
		switch sub.EventType {
		case wasi.FDReadEvent, wasi.FDWriteEvent:
			f, _, _ := s.LookupFD(sub.GetFDReadWrite().FD, 0)
			if f == nil {
				continue
			}
			if !f.FDPoll(sub.EventType, nil) {
				continue
			}
			events[i] = makePollEvent(sub)
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

var (
	_ wasi.System = (*System)(nil)
)
