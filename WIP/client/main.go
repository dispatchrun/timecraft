package main

import (
	"fmt"

	timecraft "github.com/stealthrocket/timecraft/client"
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
	fmt.Println("Running inside timecraft version", version)
}
