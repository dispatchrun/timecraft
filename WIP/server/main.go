package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/bufbuild/connect-go"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type TimecraftServer struct{}

func (s *TimecraftServer) SubmitTask(ctx context.Context, req *connect.Request[v1.SubmitTaskRequest]) (*connect.Response[v1.SubmitTaskResponse], error) {
	fmt.Println("Submitting taskgrpcurl:", req.Msg.Name)
	res := connect.NewResponse(&v1.SubmitTaskResponse{Code: 202})
	return res, nil
}

func main() {
	mux := http.NewServeMux()
	mux.Handle(serverv1connect.NewTimecraftServiceHandler(
		&TimecraftServer{},
		connect.WithCodec(grpc.Codec{}),
	))
	server := &http.Server{
		Addr:    "localhost:8080",
		Handler: h2c.NewHandler(mux, &http2.Server{}),
		// TODO: timeouts/limits
	}
	os.Remove("/tmp/timecraft.sock")
	l, err := net.Listen("unix", "/tmp/timecraft.sock")
	if err != nil {
		panic(err)
	}
	if err := server.Serve(l); err != nil {
		panic(err)
	}
}
