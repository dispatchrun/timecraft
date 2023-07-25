package sandbox

func (s *socketFD) release(fd int) {
	s.releaseFunc(fd, closeTraceError)
}

func (s *socketFD) close() {
	s.closeFunc(closeTraceError)
}
