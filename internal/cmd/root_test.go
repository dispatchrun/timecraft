package cmd_test

import (
	"fmt"
	"os"
)

func OK(rc int) {
	if rc != 0 {
		fmt.Fprintf(os.Stderr, "exit: %d\n", rc)
	}
}
