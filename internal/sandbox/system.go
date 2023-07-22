package sandbox

import (
	"context"
	"io"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"strconv"
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

// Network configures the network namespace exposed to the guest module.
//
// Default to only exposing a loopback interface.
func Network(ns Namespace) Option {
	return func(s *System) { s.netns = ns }
}

// Resolver configures the name resolver used when the guest attempts to
// lookup addresses.
//
// Default to disabling name resolution.
func Resolver(rslv ServiceResolver) Option {
	return func(s *System) { s.rslv = rslv }
}

// MaxOpenFiles configures the maximum number of files that can be opened by
// the guest module.
//
// Note that the limit applies only to files open via PathOpen or sockets
// created by SockOpen or SockAccept, it does not apply to preopens installed
// directly by the host.
//
// Default to no limits (zero).
func MaxOpenFiles(n int) Option {
	return func(s *System) { s.files.MaxOpenFiles = n }
}

// MaxOpenDirs configures the maximum number of directories that can be opened
// by the guest module.
//
// Default to no limits (zero).
func MaxOpenDirs(n int) Option {
	return func(s *System) { s.files.MaxOpenDirs = n }
}

// System is an implementation of the wasi.System interface which sandboxes all
// interactions of the guest module with the world.
type System struct {
	args   []string
	env    []string
	epoch  time.Time
	time   func() time.Time
	rand   io.Reader
	files  wasi.FileTable[File]
	stdin  *os.File
	stdout *os.File
	stderr *os.File
	root   wasi.FD
	rslv   ServiceResolver
	netns  Namespace
	mounts []mountPoint
	system
}

type mountPoint struct {
	path string
	fsys FS
}

const (
	none = ^wasi.FD(0)
)

// New creates a new System instance, applying the list of options passed as
// arguments.
func New(opts ...Option) *System {
	s, err := NewSystem(opts...)
	if err != nil {
		panic(err)
	}
	return s
}

func NewSystem(opts ...Option) (*System, error) {
	s := &System{
		root: none,
		rslv: defaultResolver{},
	}

	for _, opt := range opts {
		opt(s)
	}

	stdin, stdout, stderr, err := stdio()
	if err != nil {
		return nil, err
	}
	s.stdin = os.NewFile(stdin[1], "")
	s.stdout = os.NewFile(stdout[0], "")
	s.stderr = os.NewFile(stderr[0], "")
	setNonblock(stdin[0], false)
	setNonblock(stdout[1], false)
	setNonblock(stderr[1], false)

	s.files.Preopen(&input{fd: stdin[0]}, "/dev/stdin", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDWriteRight,
	})
	s.files.Preopen(&output{fd: stdout[1]}, "/dev/stdout", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.TTYRights & ^wasi.FDReadRight,
	})
	s.files.Preopen(&output{fd: stderr[1]}, "/dev/stderr", wasi.FDStat{
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
			s.Close(context.Background())
			return nil, &fs.PathError{Op: "open", Path: mount.path, Err: errno.Syscall()}
		}
		fd := s.files.Preopen(f, mount.path, wasi.FDStat{
			FileType:         wasi.DirectoryType,
			RightsBase:       wasi.DirectoryRights,
			RightsInheriting: wasi.DirectoryRights | wasi.FileRights,
		})
		if s.root == none {
			// TODO: this is a bit of a hack intended to pass the fstest test
			// suite, ideally we should have a mechanism to create a unified
			// view of all mount points when converting to a fs.FS.
			s.root = fd
		}
	}

	if s.time != nil {
		s.epoch = s.time()
	}
	return s, nil
}

func (s *System) Close(ctx context.Context) error {
	s.close()
	s.stdin.Close()
	s.stdout.Close()
	s.stderr.Close()
	return s.files.Close(ctx)
}

func (s *System) PreopenFD(fd wasi.FD) { s.files.PreopenFD(fd) }

// Stdin returns a writer to the standard input of the guest module.
func (s *System) Stdin() io.WriteCloser { return s.stdin }

// Stdout returns a writer to the standard output of the guest module.
func (s *System) Stdout() io.ReadCloser { return s.stdout }

// Stderr returns a writer to the standard output of the guest module.
func (s *System) Stderr() io.ReadCloser { return s.stderr }

// FS returns a fs.FS exposing the file system mounted to the guest module.
func (s *System) FS() fs.FS {
	if s.root == none {
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
	_, err := io.ReadFull(s.rand, b)
	if err != nil {
		return wasi.EIO
	}
	return wasi.ESUCCESS
}

func (s *System) FDAdvise(ctx context.Context, fd wasi.FD, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return s.files.FDAdvise(ctx, fd, offset, length, advice)
}

func (s *System) FDAllocate(ctx context.Context, fd wasi.FD, offset, length wasi.FileSize) wasi.Errno {
	return s.files.FDAllocate(ctx, fd, offset, length)
}

func (s *System) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	return s.files.FDClose(ctx, fd)
}

func (s *System) FDDataSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	return s.files.FDDataSync(ctx, fd)
}

func (s *System) FDStatGet(ctx context.Context, fd wasi.FD) (wasi.FDStat, wasi.Errno) {
	return s.files.FDStatGet(ctx, fd)
}

func (s *System) FDStatSetFlags(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) wasi.Errno {
	return s.files.FDStatSetFlags(ctx, fd, flags)
}

func (s *System) FDStatSetRights(ctx context.Context, fd wasi.FD, rightsBase, rightsInheriting wasi.Rights) wasi.Errno {
	return s.files.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
}

func (s *System) FDFileStatGet(ctx context.Context, fd wasi.FD) (wasi.FileStat, wasi.Errno) {
	return s.files.FDFileStatGet(ctx, fd)
}

func (s *System) FDFileStatSetSize(ctx context.Context, fd wasi.FD, size wasi.FileSize) wasi.Errno {
	return s.files.FDFileStatSetSize(ctx, fd, size)
}

func (s *System) FDFileStatSetTimes(ctx context.Context, fd wasi.FD, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return s.files.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
}

func (s *System) FDPread(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return s.files.FDPread(ctx, fd, iovs, offset)
}

func (s *System) FDPreStatGet(ctx context.Context, fd wasi.FD) (wasi.PreStat, wasi.Errno) {
	return s.files.FDPreStatGet(ctx, fd)
}

func (s *System) FDPreStatDirName(ctx context.Context, fd wasi.FD) (string, wasi.Errno) {
	return s.files.FDPreStatDirName(ctx, fd)
}

func (s *System) FDPwrite(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return s.files.FDPwrite(ctx, fd, iovs, offset)
}

func (s *System) FDRead(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.files.FDRead(ctx, fd, iovs)
}

func (s *System) FDReadDir(ctx context.Context, fd wasi.FD, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	return s.files.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
}

func (s *System) FDRenumber(ctx context.Context, from, to wasi.FD) wasi.Errno {
	return s.files.FDRenumber(ctx, from, to)
}

func (s *System) FDSeek(ctx context.Context, fd wasi.FD, offset wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return s.files.FDSeek(ctx, fd, offset, whence)
}

func (s *System) FDSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	return s.files.FDSync(ctx, fd)
}

func (s *System) FDTell(ctx context.Context, fd wasi.FD) (wasi.FileSize, wasi.Errno) {
	return s.files.FDTell(ctx, fd)
}

func (s *System) FDWrite(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.files.FDWrite(ctx, fd, iovs)
}

func (s *System) PathCreateDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return s.files.PathCreateDirectory(ctx, fd, path)
}

func (s *System) PathFileStatGet(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return s.files.PathFileStatGet(ctx, fd, lookupFlags, path)
}

func (s *System) PathFileStatSetTimes(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return s.files.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
}

func (s *System) PathLink(ctx context.Context, oldFD wasi.FD, oldFlags wasi.LookupFlags, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	return s.files.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
}

func (s *System) PathOpen(ctx context.Context, fd wasi.FD, dirFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (wasi.FD, wasi.Errno) {
	return s.files.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
}

func (s *System) PathReadLink(ctx context.Context, fd wasi.FD, path string, buffer []byte) (int, wasi.Errno) {
	return s.files.PathReadLink(ctx, fd, path, buffer)
}

func (s *System) PathRemoveDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return s.files.PathRemoveDirectory(ctx, fd, path)
}

func (s *System) PathRename(ctx context.Context, fd wasi.FD, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	return s.files.PathRename(ctx, fd, oldPath, newFD, newPath)
}

func (s *System) PathSymlink(ctx context.Context, oldPath string, fd wasi.FD, newPath string) wasi.Errno {
	return s.files.PathSymlink(ctx, oldPath, fd, newPath)
}

func (s *System) PathUnlinkFile(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return s.files.PathUnlinkFile(ctx, fd, path)
}

func (s *System) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	sock, stat, errno := s.files.LookupSocketFD(fd, wasi.SockAcceptRight)
	if errno != wasi.ESUCCESS {
		return none, nil, nil, errno
	}
	conn, errno := sock.SockAccept(ctx, flags)
	if errno != wasi.ESUCCESS {
		return none, nil, nil, errno
	}
	defer func() {
		if conn != nil {
			conn.FDClose(ctx)
		}
	}()
	if s.files.MaxOpenFiles > 0 && s.files.NumOpenFiles() >= s.files.MaxOpenFiles {
		return none, nil, nil, wasi.ENFILE
	}
	addr, errno := conn.SockLocalAddress(ctx)
	if errno != wasi.ESUCCESS {
		return none, nil, nil, wasi.ECONNREFUSED
	}
	peer, errno := conn.SockRemoteAddress(ctx)
	if errno != wasi.ESUCCESS {
		return none, nil, nil, wasi.ECONNREFUSED
	}
	newFD := s.files.Register(conn, wasi.FDStat{
		Flags:      flags,
		FileType:   stat.FileType,
		RightsBase: stat.RightsInheriting,
	})
	conn = nil
	return newFD, peer, addr, wasi.ESUCCESS
}

func (s *System) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return 0, 0, errno
	}
	return sock.SockRecv(ctx, iovecs, flags)
}

func (s *System) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return 0, errno
	}
	return sock.SockSend(ctx, iovecs, flags)
}

func (s *System) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	if (flags & ^(wasi.ShutdownRD | wasi.ShutdownWR)) != 0 {
		return wasi.EINVAL
	}
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return sock.SockShutdown(ctx, flags)
}

func (s *System) SockOpen(ctx context.Context, pf wasi.ProtocolFamily, st wasi.SocketType, proto wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	if st == wasi.AnySocket {
		switch proto {
		case wasi.TCPProtocol:
			st = wasi.StreamSocket
		case wasi.UDPProtocol:
			st = wasi.DatagramSocket
		default:
			return none, wasi.EPROTONOSUPPORT
		}
	}

	var protocol Protocol
	switch proto {
	case wasi.IPProtocol:
	case wasi.TCPProtocol:
		protocol = TCP
	case wasi.UDPProtocol:
		protocol = UDP
	default:
		return none, wasi.EPROTONOSUPPORT
	}

	var family Family
	switch pf {
	case wasi.InetFamily:
		family = INET
	case wasi.Inet6Family:
		family = INET6
	case wasi.UnixFamily:
		family = UNIX
	default:
		return none, wasi.EAFNOSUPPORT
	}

	if s.files.MaxOpenFiles > 0 && s.files.NumOpenFiles() >= s.files.MaxOpenFiles {
		return none, wasi.ENFILE
	}

	var sockType Socktype
	var fileType wasi.FileType
	switch st {
	case wasi.StreamSocket:
		fileType = wasi.SocketStreamType
		sockType = STREAM
	case wasi.DatagramSocket:
		fileType = wasi.SocketDGramType
		sockType = DGRAM
	}

	if s.netns == nil {
		return ^wasi.FD(0), wasi.EAFNOSUPPORT
	}

	socket, err := s.netns.Socket(family, sockType, protocol)
	if err != nil {
		return ^wasi.FD(0), wasi.MakeErrno(err)
	}

	newFD := s.files.Register(&wasiSocket{socket: socket}, wasi.FDStat{
		FileType:         fileType,
		RightsBase:       rightsBase,
		RightsInheriting: rightsInheriting,
	})
	return newFD, wasi.ESUCCESS
}

func (s *System) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.SockAcceptRight)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	if errno := sock.SockBind(ctx, addr); errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockLocalAddress(ctx)
}

func (s *System) SockConnect(ctx context.Context, fd wasi.FD, peer wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, 0)
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
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.SockAcceptRight)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return sock.SockListen(ctx, backlog)
}

func (s *System) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.FDWriteRight)
	if errno != wasi.ESUCCESS {
		return 0, errno
	}
	return sock.SockSendTo(ctx, iovecs, flags, addr)
}

func (s *System) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, wasi.FDReadRight)
	if errno != wasi.ESUCCESS {
		return 0, 0, nil, errno
	}
	return sock.SockRecvFrom(ctx, iovecs, flags)
}

func (s *System) SockGetOpt(ctx context.Context, fd wasi.FD, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockGetOpt(ctx, option)
}

func (s *System) SockSetOpt(ctx context.Context, fd wasi.FD, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	sock, _, errno := s.files.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return sock.SockSetOpt(ctx, option, value)
}

func (s *System) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, 0)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return sock.SockLocalAddress(ctx)
}

func (s *System) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	sock, _, errno := s.files.LookupSocketFD(fd, 0)
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

var (
	_ wasi.System = (*System)(nil)
)

// Dial opens a connection to a listening socket on the guest module network.
//
// This function has a signature that matches the one commonly used in the
// Go standard library as a hook to customize how and where network connections
// are estalibshed. The intent is for this function to be used when the host
// needs to establish a connection to the guest, maybe indirectly such as using
// a http.Transport and setting this method as the transport's dial function.
func (s *System) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	c, err := s.dial(ctx, network, address)
	if err != nil {
		return nil, &net.OpError{
			Op:  "dial",
			Net: network,
			Err: err,
		}
	}
	return c, nil
}

func (s *System) dial(ctx context.Context, network, address string) (net.Conn, error) {
	if s.netns == nil {
		return nil, net.UnknownNetworkError(network)
	}

	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
		addrPort, err := netip.ParseAddrPort(address)
		if err != nil {
			return nil, &net.ParseError{
				Type: "connect address",
				Text: address,
			}
		}

		addr := addrPort.Addr()
		port := addrPort.Port()
		if port == 0 {
			return nil, &net.AddrError{
				Err:  "missing port in connect address",
				Addr: address,
			}
		}

		socket, err := s.netns.Socket(socketFamily(network), socketType(network), socketProtocol(network))
		if err != nil {
			return nil, err
		}
		defer func() {
			if socket != nil {
				socket.Close()
			}
		}()

		errch := make(chan error, 1)
		go func() {
			errch <- socket.Connect(socketAddress(network, addr, port))
			close(errch)
		}()

		select {
		case err = <-errch:
		case <-ctx.Done():
			err = context.Cause(ctx)
		}
		if err != nil {
			socket.Close()
			<-errch // wait for the goroutine to terminate
			return nil, err
		}

		name, err := socket.Name()
		if err != nil {
			return nil, err
		}
		peer, err := socket.Peer()
		if err != nil {
			return nil, err
		}
		conn := &socketConn{
			sock:  socket,
			laddr: networkAddress(network, name),
			raddr: networkAddress(network, peer),
		}
		socket = nil
		return conn, nil
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

// Listen opens a listening socket on the network stack of the guest module,
// returning a net.Listener that the host can use to receive connections to the
// given network address.
//
// The returned listener does not exist in the guest module file table, which
// means that the guest cannot shut it down, allowing the host ot have full
// control over the lifecycle of the underlying socket.
func (s *System) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	l, err := s.listen(ctx, network, address)
	if err != nil {
		return nil, &net.OpError{
			Op:  "listen",
			Net: network,
			Err: err,
		}
	}
	return l, nil
}

func (s *System) listen(ctx context.Context, network, address string) (net.Listener, error) {
	if s.netns == nil {
		return nil, net.UnknownNetworkError(network)
	}

	switch network {
	case "tcp", "tcp4", "tcp6":
		addr, port, err := parseListenAddrPort(network, address)
		if err != nil {
			return nil, err
		}

		socket, err := s.netns.Socket(socketFamily(network), socketType(network), socketProtocol(network))
		if err != nil {
			return nil, err
		}
		defer func() {
			if socket != nil {
				socket.Close()
			}
		}()
		if err := socket.Bind(socketAddress(network, addr, port)); err != nil {
			return nil, err
		}
		if err := socket.Listen(128); err != nil {
			return nil, err
		}
		name, err := socket.Name()
		if err != nil {
			return nil, err
		}

		listener := &socketListener{
			sock: socket,
			addr: networkAddress(network, name),
		}
		socket = nil
		return listener, nil
	case "unix":
		return nil, net.UnknownNetworkError(network)
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

// ListenPacket is like Listen but for datagram connections.
//
// The supported networks are "udp", "udp4", and "udp6".
func (s *System) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	c, err := s.listenPacket(ctx, network, address)
	if err != nil {
		return nil, &net.OpError{
			Op:  "listen",
			Net: network,
			Err: err,
		}
	}
	return c, nil
}

func (s *System) listenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	if s.netns == nil {
		return nil, net.UnknownNetworkError(network)
	}

	switch network {
	case "udp", "udp4", "udp6":
		addr, port, err := parseListenAddrPort(network, address)
		if err != nil {
			return nil, err
		}

		socket, err := s.netns.Socket(socketFamily(network), socketType(network), socketProtocol(network))
		if err != nil {
			return nil, err
		}
		defer func() {
			if socket != nil {
				socket.Close()
			}
		}()
		if err := socket.Bind(socketAddress(network, addr, port)); err != nil {
			return nil, err
		}
		name, err := socket.Name()
		if err != nil {
			return nil, err
		}

		conn := &socketConn{
			sock:  socket,
			laddr: networkAddress(network, name),
		}
		socket = nil
		return conn, nil
	default:
		return nil, net.UnknownNetworkError(network)
	}
}

func parseListenAddrPort(network, address string) (addr netip.Addr, port uint16, err error) {
	h, p, err := net.SplitHostPort(address)
	if err != nil {
		return addr, port, &net.ParseError{
			Type: "listen address",
			Text: address,
		}
	}

	// Allow omitting the address to let the system select the best match.
	if h == "" {
		if network == "tcp6" || network == "udp6" {
			h = "[::]"
		} else {
			h = "0.0.0.0"
		}
	}

	addrPort, err := netip.ParseAddrPort(net.JoinHostPort(h, p))
	if err != nil {
		return addr, port, &net.ParseError{
			Type: "listen address",
			Text: address,
		}
	}

	addr = addrPort.Addr()
	port = addrPort.Port()

	if addr.Is4() {
		if network == "tcp6" || network == "udp6" {
			err = net.InvalidAddrError(address)
		}
	} else {
		if network == "tcp4" || network == "udp4" {
			err = net.InvalidAddrError(address)
		}
	}

	return addr, port, err
}

func socketFamily(network string) Family {
	switch network {
	case "unix", "unixgram", "unixpacket":
		return UNIX
	case "tcp4", "udp4":
		return INET
	default:
		return INET6
	}
}

func socketType(network string) Socktype {
	switch network {
	case "unix", "tcp", "tcp4", "tcp6":
		return STREAM
	default:
		return DGRAM
	}
}

func socketProtocol(network string) Protocol {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return TCP
	case "udp", "udp4", "udp6":
		return UDP
	default:
		return 0
	}
}

func socketAddress(network string, addr netip.Addr, port uint16) Sockaddr {
	switch socketFamily(network) {
	case INET:
		return &SockaddrInet4{
			Addr: addr.As4(),
			Port: int(port),
		}
	default:
		return &SockaddrInet6{
			Addr: addr.As16(),
			Port: int(port),
		}
	}
}

func networkAddress(network string, sa Sockaddr) net.Addr {
	var addrPort netip.AddrPort
	switch a := sa.(type) {
	case *SockaddrInet4:
		addrPort = addrPortFromInet4(a)
	case *SockaddrInet6:
		addrPort = addrPortFromInet6(a)
	case *SockaddrUnix:
		return &net.UnixAddr{
			Net:  network,
			Name: a.Name,
		}
	default:
		return nil
	}
	addr := addrPort.Addr()
	port := addrPort.Port()
	switch socketType(network) {
	case STREAM:
		return &net.TCPAddr{
			Port: int(port),
			IP:   addr.AsSlice(),
			Zone: addr.Zone(),
		}
	default:
		return &net.UDPAddr{
			Port: int(port),
			IP:   addr.AsSlice(),
			Zone: addr.Zone(),
		}
	}
}
