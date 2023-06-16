package timecraft

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/google/uuid"
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
		&grpcServer{instance: t},
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
	serverv1connect.UnimplementedTimecraftServiceHandler

	instance *Server
}

func (s *grpcServer) Spawn(ctx context.Context, req *connect.Request[v1.SpawnRequest]) (*connect.Response[v1.SpawnResponse], error) {
	moduleSpec := s.instance.Module // shallow copy
	if req.Msg.Path != "" {
		moduleSpec.Path = req.Msg.Path
	}
	moduleSpec.Args = req.Msg.Args
	moduleSpec.Dials = nil   // not supported
	moduleSpec.Listens = nil // not supported

	childID := uuid.New()
	var logSpec *LogSpec
	if parentLog := s.instance.Log; parentLog != nil {
		logSpec = &LogSpec{
			ProcessID:   childID,
			StartTime:   time.Now(),
			Compression: parentLog.Compression,
			BatchSize:   parentLog.BatchSize,
		}
	}

	childModule, err := s.instance.Runner.Prepare(moduleSpec, logSpec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to prepare module: %w", err))
	}

	go func() {
		defer childModule.Close()

		if err := s.instance.Runner.RunModule(childModule); err != nil {
			panic(err) // TODO: handle error
		}

		// TODO: need to prevent the parent from exiting until now, unless the server has
		//  been cancelled via signal
	}()

	res := connect.NewResponse(&v1.SpawnResponse{TaskId: childID.String()})
	return res, nil
}

func (s *grpcServer) Version(ctx context.Context, req *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	res := connect.NewResponse(&v1.VersionResponse{Version: s.instance.Version})
	return res, nil
}
