package main

import (
	"context"
	"fmt"
	"os"
)

// declare main because we have to but we don't use it.
func main() {}

//go:wasm-module timecraft
//export fork
func fork() int32

// spanw creates a fork and only execute the child branch
func spawn(ctx context.Context, fn func(ctx context.Context) error) {
	if fork() == 0 {
		if err := fn(ctx); err != nil {
			os.Exit(-1) //TODO: better error handling
		}
	}
}

var (
	exitCode = 0
)

//export forkAndExit
func forkAndExit() {
	exitCode += 1
	if fork() == 0 {
		exitCode += 1
	} else {
		exitCode += 2
	}
	os.Exit(exitCode)
}

//export spawnWithWasi
func spawnWithWasi() {
	ctx := context.Background()

	n := 42

	spawn(ctx, func(ctx context.Context) error {
		stdout := os.Stdout
		_, err := stdout.WriteString(fmt.Sprintf("%d child\n", n))
		return err
	})
}

//export forkWithWasi
func forkWithWasi() {
	stdout := os.Stdout

	if fork() == 0 {
		_, err := stdout.WriteString("hello from child\n")
		if err != nil {
			os.Exit(-1)
		}
	} else {
		_, err := stdout.WriteString("hello from main\n")
		if err != nil {
			os.Exit(-1)
		}
	}
}

//export forkAndPrint
func forkAndPrint() {
	if fork() == 0 {
		fmt.Println("hello from child")
	} else {
		fmt.Println("hello from main")
	}
}
