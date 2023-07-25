package sandbox

func (f *fdRef) release(fd int) {
	f.releaseFunc(fd, closeTraceError)
}

func (f *fdRef) close() {
	f.closeFunc(closeTraceError)
}
