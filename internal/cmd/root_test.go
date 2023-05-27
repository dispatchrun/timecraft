package cmd_test

import (
	"fmt"
	"os"
)

func PASS(rc int) {
	if rc != 0 {
		fmt.Fprintf(os.Stderr, "exit: %d\n", rc)
	}
}

func FAIL(rc int) {
	if rc != 1 {
		fmt.Fprintf(os.Stderr, "exit: %d\n", rc)
	}
}
