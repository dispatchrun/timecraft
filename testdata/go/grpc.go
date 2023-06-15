package main

import (
	"fmt"

	"github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

func main() {
	c, err := timecraft.NewClient()
	if err != nil {
		panic(err)
	}
	version, err := c.Version()
	if err != nil {
		panic(err)
	}
	fmt.Println(version)
}
