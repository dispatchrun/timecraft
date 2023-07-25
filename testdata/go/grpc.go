package main

import (
	"context"
	"fmt"
	"log"

	"github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

func main() {
	c, err := timecraft.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	version, err := c.Version(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(version)
}
