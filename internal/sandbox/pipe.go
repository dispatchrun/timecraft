package sandbox

import (
	"context"

	"github.com/stealthrocket/wasi-go"
	wasisys "github.com/stealthrocket/wasi-go/systems/unix"
)

func stdio() (stdin, stdout, stderr [2]uintptr, err error) {
	fds0 := [2]int{-1, -1}
	fds1 := [2]int{-1, -1}
	fds2 := [2]int{-1, -1}

	defer func() {
		closePair(&fds0)
		closePair(&fds1)
		closePair(&fds2)
	}()

	if err = pipe(&fds0); err != nil {
		return
	}
	if err = pipe(&fds1); err != nil {
		return
	}
	if err = pipe(&fds2); err != nil {
		return
	}

	stdin[0] = uintptr(fds0[0])
	stdin[1] = uintptr(fds0[1])

	stdout[0] = uintptr(fds1[0])
	stdout[1] = uintptr(fds1[1])

	stderr[0] = uintptr(fds2[0])
	stderr[1] = uintptr(fds2[1])

	fds0 = [2]int{-1, -1}
	fds1 = [2]int{-1, -1}
	fds2 = [2]int{-1, -1}
	return
}

type input struct {
	unimplementedFileMethods
	unimplementedSocketMethods
	fd uintptr
}

func (in *input) Fd() uintptr {
	return in.fd
}

func (in *input) FDClose(ctx context.Context) wasi.Errno {
	return wasisys.FD(in.fd).FDClose(ctx)
}

func (in *input) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return wasisys.FD(in.fd).FDRead(ctx, iovs)
}

func (in *input) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return ^wasi.Size(0), wasi.EBADF
}

func (in *input) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return wasisys.FD(in.fd).FDStatSetFlags(ctx, flags)
}

func (in *input) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	stat := wasi.FileStat{FileType: wasi.CharacterDeviceType}
	return stat, wasi.ESUCCESS
}

type output struct {
	unimplementedFileMethods
	unimplementedSocketMethods
	fd uintptr
}

func (out *output) Fd() uintptr {
	return out.fd
}

func (out *output) FDClose(ctx context.Context) wasi.Errno {
	return wasisys.FD(out.fd).FDClose(ctx)
}

func (out *output) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return ^wasi.Size(0), wasi.EBADF
}

func (out *output) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return wasisys.FD(out.fd).FDWrite(ctx, iovs)
}

func (out *output) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return wasisys.FD(out.fd).FDStatSetFlags(ctx, flags)
}

func (out *output) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	stat := wasi.FileStat{FileType: wasi.CharacterDeviceType}
	return stat, wasi.ESUCCESS
}
