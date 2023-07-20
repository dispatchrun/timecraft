package chaos_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/chaos"
	"github.com/stealthrocket/wasi-go"
)

type yieldSystem struct {
	wasi.System
	yield int
}

func (s *yieldSystem) SchedYield(ctx context.Context) wasi.Errno {
	s.yield++
	return wasi.ESUCCESS
}

func TestRandomSystemSelection(t *testing.T) {
	s1 := new(yieldSystem)
	s2 := new(yieldSystem)
	s3 := new(yieldSystem)

	prng := rand.NewSource(0)
	sys := chaos.New(prng, s1, chaos.Chance(0.1, s2), chaos.Chance(0.2, s3))
	ctx := context.Background()

	const N = 1e6
	for i := 0; i < N; i++ {
		errno := sys.SchedYield(ctx)
		assert.Equal(t, errno, wasi.ESUCCESS)
	}

	const epsilon = 0.01
	assert.FloatEqual(t, float64(s1.yield)/N, 0.7, epsilon)
	assert.FloatEqual(t, float64(s2.yield)/N, 0.1, epsilon)
	assert.FloatEqual(t, float64(s3.yield)/N, 0.2, epsilon)
}
