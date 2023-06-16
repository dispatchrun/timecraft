package timecraft

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

// Server is a gRPC server that's available to guests.
type Server struct {
	// Runner is the Runner used to run the WebAssembly module that this
	// server instance is serving.
	Runner *Runner

	// Module is details about the WebAssembly module that this server
	// instance is serving.
	Module ModuleSpec

	// Log, if non-nil, is details about the recorded trace of execution.
	// If nil, it indicates that the WebAssembly module is not currently
	// being recorded.
	Log *LogSpec

	// Version is the timecraft version reported by the server.
	Version string
}

// Serve serves using the specified net.Listener.
func (t *Server) Serve(l net.Listener) error {
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
	t *Server
}

func (s *grpcServer) Version(ctx context.Context, req *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	res := connect.NewResponse(&v1.VersionResponse{Version: s.t.Version})
	return res, nil
}
