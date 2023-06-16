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
		&grpcServer{timecraft: t},
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

	timecraft *Server
}

func (s *grpcServer) Spawn(ctx context.Context, req *connect.Request[v1.SpawnRequest]) (*connect.Response[v1.SpawnResponse], error) {
	child := s.timecraft.Module.Copy()
	if req.Msg.Path != "" {
		child.Path = req.Msg.Path
	}
	child.Args = req.Msg.Args
	child.Dials = nil   // not supported
	child.Listens = nil // not supported

	childModule, err := s.timecraft.Runner.Prepare(child)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to prepare module: %w", err))
	}

	childID := uuid.New()
	if log := s.timecraft.Log; log != nil {
		childLog := *log
		childLog.ProcessID = childID
		childLog.StartTime = time.Now()
		err = s.timecraft.Runner.PrepareLog(childModule, childLog)
		if err != nil {
			childModule.Close()
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to prepare log: %w", err))
		}
	}

	go func() {
		defer childModule.Close()

		if err := s.timecraft.Runner.RunModule(childModule); err != nil {
			panic(err) // TODO: handle error
		}

		// TODO: need to prevent the parent from exiting until now, unless the server has
		//  been cancelled via signal
	}()

	res := connect.NewResponse(&v1.SpawnResponse{TaskId: childID.String()})
	return res, nil
}

func (s *grpcServer) Version(ctx context.Context, req *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	res := connect.NewResponse(&v1.VersionResponse{Version: s.timecraft.Version})
	return res, nil
}
