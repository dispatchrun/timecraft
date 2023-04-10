package testdata

import (
	"runtime/timecraft"
	"sync"
	"testing"
	"time"
)

func TestPromiseResolveOne(t *testing.T) {
	p := timecraft.NewPromise()

	select {
	case <-p:
		t.Fatal("promise already resolved right after creation")
	default:
	}

	p.Resolve()

	// blocks indefinitly if resolution did not work
	<-p
}

func TestPromiseResolveMany(t *testing.T) {
	promises := make([]timecraft.Promise, 100)
	for i := range promises {
		promises[i] = timecraft.NewPromise()
	}

	wg := sync.WaitGroup{}
	wg.Add(len(promises))

	for _, p := range promises {
		go func(p timecraft.Promise) {
			defer wg.Done()
			<-p
		}(p)
	}

	for _, p := range promises {
		p.Resolve()
	}

	wg.Wait()
}

func TestPromiseDiscard(t *testing.T) {
	p := timecraft.NewPromise()
	c := time.After(100 * time.Millisecond)

	p.Discard()

	select {
	case <-c:
	case <-p:
		t.Error("rejected promise has resolved")
	}
}
