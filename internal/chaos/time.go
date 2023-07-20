package chaos

import (
	"context"
	"math/rand"
	"time"

	"github.com/stealthrocket/wasi-go"
)

const (
	// Clocks will be reset roughly every minute; note that this is subject to
	// fluctuations from the underlying system clocks that the system gets the
	// current timestamps from.
	driftResetDelay = wasi.Timestamp(time.Minute)
)

// ClockDrift wraps the base system to return one where the clock is drifting
// by a random factor.
func ClockDrift(base wasi.System) wasi.System {
	return &clockDriftSystem{System: base}
}

type clockDriftSystem struct {
	wasi.System
	prng  *rand.Rand
	drift float64
	clock [4]clock
	reset []clockSubscription
}

type clock struct {
	epoch wasi.Timestamp
	errno wasi.Errno
}

type clockSubscription struct {
	i int
	s wasi.Subscription
}

func (s *clockDriftSystem) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	t, errno := s.System.ClockTimeGet(ctx, wasi.Monotonic, 1)
	if errno != wasi.ESUCCESS {
		// We need the underlying system to support monotonic clocks to
		// derive the drift on realtime clocks.
		return t, errno
	}

	i := int(id)
	if i < 0 || i >= len(s.clock) {
		// We only support the 4 clocks defined in the WASI preview 1
		// specification.
		return 0, wasi.ENOSYS
	}

	// When the timestamp is before the next reset point of the clock,
	// return the timestamp adjusted by the drift.
	if m := &s.clock[wasi.Monotonic]; m.epoch != 0 && t < m.epoch+driftResetDelay {
		d := float64(t-m.epoch) * s.drift
		c := &s.clock[i]
		return c.epoch + wasi.Timestamp(d), c.errno
	}

	// Lazily allocate the random number generator using the current
	// timestmap as seed, this makes for a good-enough seed selection
	// without having to explicitly expose configuration. This remains
	// deterministic as long as the underlying system is.
	if s.prng == nil {
		s.prng = rand.New(rand.NewSource(int64(t)))
	}
	// Each time the clocks get reset we compute a new drift to emulate a
	// clock skew correction not being completely accurate.
	s.drift = 1.0 + ((0.2 * s.prng.Float64()) - 0.1)

	for i := range s.clock {
		epoch, errno := s.System.ClockTimeGet(ctx, wasi.ClockID(i), 1)
		s.clock[i].epoch = epoch
		s.clock[i].errno = errno
	}

	c := &s.clock[i]
	return c.epoch, c.errno
}

func (s *clockDriftSystem) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	s.reset = s.reset[:0]

	// Apply the reverse drift to absolute clock events found in the
	// subscription array since the application may have miscalculated
	// those.
	for i, sub := range subscriptions {
		if sub.EventType != wasi.ClockEvent {
			continue
		}
		clockEvent := sub.GetClock()
		if !clockEvent.Flags.Has(wasi.Abstime) {
			continue
		}
		clockID := int(clockEvent.ID)
		if clockID < 0 || clockID >= len(s.clock) {
			continue
		}
		c := &s.clock[clockID]
		if c.errno != wasi.ESUCCESS {
			continue
		}
		d := float64(clockEvent.Timeout-c.epoch) * -s.drift
		clockEvent.Timeout = c.epoch + wasi.Timestamp(d)
		subscriptions[i] = wasi.MakeSubscriptionClock(sub.UserData, clockEvent)
		s.reset = append(s.reset, clockSubscription{i, sub})
	}

	defer func() {
		// Reset the timeouts we altered so the application does not know that
		// we mutated the subscription array.
		for _, reset := range s.reset {
			subscriptions[reset.i] = reset.s
		}
	}()

	return s.System.PollOneOff(ctx, subscriptions, events)
}
