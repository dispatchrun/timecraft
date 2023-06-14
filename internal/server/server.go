package server

import (
	"context"
	"net"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// TimecraftServer is a gRPC server that's available to guests.
type TimecraftServer struct {
	Version string
}

// Serve serves the TimecraftServer using the specified net.Listener.
func (t *TimecraftServer) Serve(l net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle(serverv1connect.NewTimecraftServiceHandler(
		&grpcServer{t},
		connect.WithCodec(grpc.Codec{}),
	))
	server := &http.Server{
		Addr:    "timecraft",
		Handler: h2c.NewHandler(mux, &http2.Server{}),
		// TODO: timeouts/limits
	}
	return server.Serve(l)
}

type grpcServer struct {
	t *TimecraftServer
}

func (s *grpcServer) Version(ctx context.Context, req *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	res := connect.NewResponse(&v1.VersionResponse{Version: s.t.Version})
	return res, nil
}
