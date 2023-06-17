package main

import (
	"context"
	"fmt"

	"github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

func main() {
	c, err := timecraft.NewClient()
	if err != nil {
		panic(err)
	}
	version, err := c.Version(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
