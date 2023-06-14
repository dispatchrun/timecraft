package main

import (
	"net"
	"os"

	"github.com/stealthrocket/timecraft/client"
	"github.com/stealthrocket/timecraft/internal/server"
)

func main() {
	s := &server.TimecraftServer{Version: "devel"}

	os.Remove(client.Socket)
	l, err := net.Listen("unix", client.Socket)
	if err != nil {
		panic(err)
	}
	panic(s.Serve(l))
}
