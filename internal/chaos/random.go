package chaos

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// LowEntropy constructs a system with a low entropy source for random data
// generation. The seed data is used to generate random data that is not safe
// for cryptographic use and will likely cause repeatitions in the pattern of
// random data exposed to a guest program.
func LowEntropy(base wasi.System) wasi.System {
	return &randomSystem{System: base, off: -1}
}

type randomSystem struct {
	wasi.System
	off  int
	seed [1024]byte
}

func (s *randomSystem) RandomGet(ctx context.Context, data []byte) wasi.Errno {
	if s.off < 0 {
		// Lazily initialize the seed data from the underlying system; this
		// ensures that we won't produce random data if the system did not
		// suport it.
		if errno := s.System.RandomGet(ctx, s.seed[:]); errno != wasi.ESUCCESS {
			return errno
		}
		s.off = 0
	}
	for len(data) > 0 {
		data = data[copy(data, s.seed[s.off:]):]
		s.off = (s.off + 1) % len(s.seed)
	}
	return wasi.ESUCCESS
}
