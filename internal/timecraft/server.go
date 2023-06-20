package timecraft

import (
	"context"
	"net"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
)

// ServerFactory is used to create Server instances.
type ServerFactory struct {
	Scheduler *TaskScheduler
}

// NewServer creates a new Server.
func (f *ServerFactory) NewServer(processID uuid.UUID, moduleSpec ModuleSpec, logSpec *LogSpec) *Server {
	return &Server{
		scheduler:  f.Scheduler,
		processID:  processID,
		moduleSpec: moduleSpec,
		logSpec:    logSpec,
	}
}

// Server is a gRPC server that's available to guests. Every
// WebAssembly module has its own instance of a gRPC server.
type Server struct {
	serverv1connect.UnimplementedTimecraftServiceHandler

	scheduler  *TaskScheduler
	processID  uuid.UUID
	moduleSpec ModuleSpec
	logSpec    *LogSpec
}

// Serve serves using the specified net.Listener.
func (s *Server) Serve(l net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle(serverv1connect.NewTimecraftServiceHandler(
		s,
		connect.WithCompression("gzip", nil, nil), // disable gzip for now
		connect.WithCodec(grpc.Codec{}),
	))
	server := &http.Server{
		Addr:    "timecraft",
		Handler: mux,
		// TODO: timeouts/limits
	}
	return server.Serve(l)
}

func (s *Server) SubmitTask(ctx context.Context, req *connect.Request[v1.SubmitTaskRequest]) (*connect.Response[v1.SubmitTaskResponse], error) {
	panic("not implemented")
}

func (s *Server) Version(context.Context, *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	return connect.NewResponse(&v1.VersionResponse{Version: Version()}), nil
}
