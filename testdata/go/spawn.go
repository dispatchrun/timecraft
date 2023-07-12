//go:build wasip1

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

func main() {
	var err error
	switch {
	case len(os.Args) == 2 && os.Args[1] == "worker":
		err = worker()
	case len(os.Args) == 1:
		err = supervisor(context.Background())
	default:
		err = fmt.Errorf("usage: task.wasm [worker]")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}

func supervisor(ctx context.Context) error {
	client, err := timecraft.NewClient()
	if err != nil {
		return fmt.Errorf("failed to connect to timecraft: %w", err)
	}

	version, err := client.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve version: %w", err)
	}
	processID, err := client.ProcessID(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve process ID: %w", err)
	}
	fmt.Printf("executing as process %s in timecraft %s\n", processID, version)

	// Spawn the same WASM module, but with the "worker" arg.
	workerModule := timecraft.ModuleSpec{Args: []string{"worker"}}

	workerID, err := client.Spawn(ctx, workerModule)
	if err != nil {
		return fmt.Errorf("failed to spawn worker: %w", err)
	}

	fmt.Printf("spawned worker process %s\n", workerID)

	// FIXME:
	time.Sleep(5 * time.Second)

	return nil
}

func worker() error {
	fmt.Println("hello from worker!")
	return nil
}
