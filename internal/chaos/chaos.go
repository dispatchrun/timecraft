package chaos

import (
	"context"
	"math"
	"math/rand"

	"github.com/stealthrocket/wasi-go"
)

const (
	maxChance = 1024 * 1024 * 1024
)

type Rule struct {
	chance int64 // [0;maxChance]
	system wasi.System
}

// Chance is a constructor for values of type Rule, mapping a system to a
// probably of it being picked when methods are invoked.
func Chance(chance float64, system wasi.System) Rule {
	if chance < 0 || chance > 1 {
		panic("invalid chance of chaos system rule is not in the range [0;1]")
	}
	return Rule{
		chance: int64(math.Round(chance * maxChance)),
		system: system,
	}
}

// New constructs a new chaos system. The given random source is used for all
// random number generation. The base system is the fallback when a probability
// of method invocation did not match any of the rules. The list of rules pairs
// a probability of being selected when a method of the system is invoked to the
// system to delegate the invocation to. The systems set on rules are expected
// to be instances of wrappers created by functions of this package to perform
// fault injection.
func New(prng rand.Source, base wasi.System, rules ...Rule) wasi.System {
	s := &system{
		prng:    prng,
		base:    base,
		chances: make([]int64, len(rules)),
		systems: make([]wasi.System, len(rules)),
	}
	cumulativeChance := int64(0)
	for i, rule := range rules {
		cumulativeChance += rule.chance
		s.chances[i] = cumulativeChance
		s.systems[i] = rule.system
		if cumulativeChance < 0 || cumulativeChance > maxChance {
			panic("cumulative chance of chaos system rules is greater than 1")
		}
	}
	return s
}

type system struct {
	prng    rand.Source
	base    wasi.System
	chances []int64
	systems []wasi.System
}

func (s *system) system() wasi.System {
	probability := s.prng.Int63() & (maxChance - 1)
	for i, chance := range s.chances {
		if chance > probability {
			return s.systems[i]
		}
	}
	return s.base
}

func (s *system) ArgsSizesGet(ctx context.Context) (int, int, wasi.Errno) {
	return s.system().ArgsSizesGet(ctx)
}

func (s *system) ArgsGet(ctx context.Context) ([]string, wasi.Errno) {
	return s.system().ArgsGet(ctx)
}

func (s *system) EnvironSizesGet(ctx context.Context) (int, int, wasi.Errno) {
	return s.system().EnvironSizesGet(ctx)
}

func (s *system) EnvironGet(ctx context.Context) ([]string, wasi.Errno) {
	return s.system().EnvironGet(ctx)
}

func (s *system) ClockResGet(ctx context.Context, id wasi.ClockID) (wasi.Timestamp, wasi.Errno) {
	return s.system().ClockResGet(ctx, id)
}

func (s *system) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	return s.system().ClockTimeGet(ctx, id, precision)
}

func (s *system) FDAdvise(ctx context.Context, fd wasi.FD, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return s.system().FDAdvise(ctx, fd, offset, length, advice)
}

func (s *system) FDAllocate(ctx context.Context, fd wasi.FD, offset, length wasi.FileSize) wasi.Errno {
	return s.system().FDAllocate(ctx, fd, offset, length)
}

func (s *system) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	return s.system().FDClose(ctx, fd)
}

func (s *system) FDDataSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	return s.system().FDDataSync(ctx, fd)
}

func (s *system) FDStatGet(ctx context.Context, fd wasi.FD) (wasi.FDStat, wasi.Errno) {
	return s.system().FDStatGet(ctx, fd)
}

func (s *system) FDStatSetFlags(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) wasi.Errno {
	return s.system().FDStatSetFlags(ctx, fd, flags)
}

func (s *system) FDStatSetRights(ctx context.Context, fd wasi.FD, rightsBase, rightsInheriting wasi.Rights) wasi.Errno {
	return s.system().FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
}

func (s *system) FDFileStatGet(ctx context.Context, fd wasi.FD) (wasi.FileStat, wasi.Errno) {
	return s.system().FDFileStatGet(ctx, fd)
}

func (s *system) FDFileStatSetSize(ctx context.Context, fd wasi.FD, size wasi.FileSize) wasi.Errno {
	return s.system().FDFileStatSetSize(ctx, fd, size)
}

func (s *system) FDFileStatSetTimes(ctx context.Context, fd wasi.FD, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return s.system().FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
}

func (s *system) FDPread(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return s.system().FDPread(ctx, fd, iovecs, offset)
}

func (s *system) FDPreStatGet(ctx context.Context, fd wasi.FD) (wasi.PreStat, wasi.Errno) {
	return s.system().FDPreStatGet(ctx, fd)
}

func (s *system) FDPreStatDirName(ctx context.Context, fd wasi.FD) (string, wasi.Errno) {
	return s.system().FDPreStatDirName(ctx, fd)
}

func (s *system) FDPwrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return s.system().FDPwrite(ctx, fd, iovecs, offset)
}

func (s *system) FDRead(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.system().FDRead(ctx, fd, iovecs)
}

func (s *system) FDReadDir(ctx context.Context, fd wasi.FD, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	return s.system().FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
}

func (s *system) FDRenumber(ctx context.Context, from, to wasi.FD) wasi.Errno {
	return s.system().FDRenumber(ctx, from, to)
}

func (s *system) FDSeek(ctx context.Context, fd wasi.FD, offset wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return s.system().FDSeek(ctx, fd, offset, whence)
}

func (s *system) FDSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	return s.system().FDSync(ctx, fd)
}

func (s *system) FDTell(ctx context.Context, fd wasi.FD) (wasi.FileSize, wasi.Errno) {
	return s.system().FDTell(ctx, fd)
}

func (s *system) FDWrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.system().FDWrite(ctx, fd, iovecs)
}

func (s *system) PathCreateDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return s.system().PathCreateDirectory(ctx, fd, path)
}

func (s *system) PathFileStatGet(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return s.system().PathFileStatGet(ctx, fd, lookupFlags, path)
}

func (s *system) PathFileStatSetTimes(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return s.system().PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
}

func (s *system) PathLink(ctx context.Context, oldFD wasi.FD, oldFlags wasi.LookupFlags, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	return s.system().PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
}

func (s *system) PathOpen(ctx context.Context, fd wasi.FD, dirFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (wasi.FD, wasi.Errno) {
	return s.system().PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
}

func (s *system) PathReadLink(ctx context.Context, fd wasi.FD, path string, buffer []byte) (int, wasi.Errno) {
	return s.system().PathReadLink(ctx, fd, path, buffer)
}

func (s *system) PathRemoveDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return s.system().PathRemoveDirectory(ctx, fd, path)
}

func (s *system) PathRename(ctx context.Context, fd wasi.FD, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	return s.system().PathRename(ctx, fd, oldPath, newFD, newPath)
}

func (s *system) PathSymlink(ctx context.Context, oldPath string, fd wasi.FD, newPath string) wasi.Errno {
	return s.system().PathSymlink(ctx, oldPath, fd, newPath)
}

func (s *system) PathUnlinkFile(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return s.system().PathUnlinkFile(ctx, fd, path)
}

func (s *system) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	// We don't generate an error from PollOneOff itself because it usually
	// results in crashing the application runtime. Instead, we injected errors
	// on the collected events.
	return s.system().PollOneOff(ctx, subscriptions, events)
}

func (s *system) ProcExit(ctx context.Context, exitCode wasi.ExitCode) wasi.Errno {
	return s.system().ProcExit(ctx, exitCode)
}

func (s *system) ProcRaise(ctx context.Context, signal wasi.Signal) wasi.Errno {
	return s.system().ProcRaise(ctx, signal)
}

func (s *system) SchedYield(ctx context.Context) wasi.Errno {
	return s.system().SchedYield(ctx)
}

func (s *system) RandomGet(ctx context.Context, b []byte) wasi.Errno {
	return s.system().RandomGet(ctx, b)
}

func (s *system) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	return s.system().SockAccept(ctx, fd, flags)
}

func (s *system) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	return s.system().SockShutdown(ctx, fd, flags)
}

func (s *system) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, iflags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return s.system().SockRecv(ctx, fd, iovecs, iflags)
}

func (s *system) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, iflags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return s.system().SockSend(ctx, fd, iovecs, iflags)
}

func (s *system) SockOpen(ctx context.Context, pf wasi.ProtocolFamily, socketType wasi.SocketType, protocol wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	return s.system().SockOpen(ctx, pf, socketType, protocol, rightsBase, rightsInheriting)
}

func (s *system) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return s.system().SockBind(ctx, fd, addr)
}

func (s *system) SockConnect(ctx context.Context, fd wasi.FD, peer wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return s.system().SockConnect(ctx, fd, peer)
}

func (s *system) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	return s.system().SockListen(ctx, fd, backlog)
}

func (s *system) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, iflags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return s.system().SockSendTo(ctx, fd, iovecs, iflags, addr)
}

func (s *system) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, iflags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return s.system().SockRecvFrom(ctx, fd, iovecs, iflags)
}

func (s *system) SockGetOpt(ctx context.Context, fd wasi.FD, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return s.system().SockGetOpt(ctx, fd, option)
}

func (s *system) SockSetOpt(ctx context.Context, fd wasi.FD, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return s.system().SockSetOpt(ctx, fd, option, value)
}

func (s *system) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	return s.system().SockLocalAddress(ctx, fd)
}

func (s *system) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	return s.system().SockRemoteAddress(ctx, fd)
}

func (s *system) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	return s.system().SockAddressInfo(ctx, name, service, hints, results)
}

func (s *system) Close(ctx context.Context) error {
	for _, system := range s.systems {
		_ = system.Close(ctx)
	}
	return s.base.Close(ctx)
}
