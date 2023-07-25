//go:build wasip1

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/stealthrocket/net/wasip1"
	"github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

const workerPort = 10948

func main() {
	var err error
	switch {
	case len(os.Args) == 2 && os.Args[1] == "worker":
		err = worker()
	case len(os.Args) == 1:
		err = supervisor(context.Background())
	default:
		err = fmt.Errorf("usage: spawn.wasm [worker]")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
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
	workerModule := timecraft.ModuleSpec{
		Args: []string{"worker"},
	}

	workerID, workerIP, err := client.Spawn(ctx, workerModule)
	if err != nil {
		return fmt.Errorf("failed to spawn worker: %w", err)
	}
	defer client.Kill(ctx, workerID)

	workerAddr := net.JoinHostPort(workerIP.String(), strconv.Itoa(workerPort))

	fmt.Printf("connecting to worker process %s on address %s\n", workerID, workerAddr)

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				var d wasip1.Dialer
				const delay = 300 * time.Millisecond
				const attempts = 10 // try for 3 seconds
				for i := 0; i < attempts; i++ {
					conn, err = d.DialContext(ctx, network, addr)
					if err == nil {
						break
					}
					time.Sleep(delay)
				}
				return
			},
		},
	}

	u, _ := url.Parse(fmt.Sprintf("http://%s/", workerAddr))
	req := &http.Request{
		Method: "GET",
		URL:    u,
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact worker: %w", err)
	}

	fmt.Printf("worker responded with %d\n", res.StatusCode)
	if res.StatusCode != 200 {
		return fmt.Errorf("unexpected worker status code: %d", res.StatusCode)
	}
	return nil
}

func worker() error {
	workerAddr := fmt.Sprintf(":%d", workerPort)
	listener, err := wasip1.Listen("tcp", workerAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", workerAddr, err)
	}
	return http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
}
