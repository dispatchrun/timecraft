//go:build wasip1

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"time"

	"github.com/stealthrocket/net/wasip1"
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
		err = fmt.Errorf("usage: spawn.wasm [worker]")
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

	workerID, workerAddr, err := client.Spawn(ctx, workerModule)
	if err != nil {
		return fmt.Errorf("failed to spawn worker: %w", err)
	}
	defer client.Kill(ctx, workerID)

	fmt.Printf("spawned worker process %s with address %s\n", workerID, workerAddr)

	// FIXME:
	time.Sleep(2 * time.Second)

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d wasip1.Dialer
				return d.DialContext(ctx, network, addr)
			},
		},
	}

	u, _ := url.Parse(fmt.Sprintf("http://%s/", netip.AddrPortFrom(workerAddr, 3000)))
	req := &http.Request{
		Method: "GET",
		URL:    u,
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to contact worker: %w", err)
	}

	fmt.Printf("worker responded with %d\n", res.StatusCode)

	return nil
}

func worker() error {
	defer fmt.Println("worker exiting")

	addr := ":3000"
	listener, err := wasip1.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	fmt.Println("worker listening on", addr)
	return http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
}
