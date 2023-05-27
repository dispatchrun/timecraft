package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/stealthrocket/timecraft/internal/cmd"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	os.Exit(cmd.Root(ctx, os.Args[1:]...))
}
