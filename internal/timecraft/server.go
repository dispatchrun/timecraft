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
)

// moduleServer is a gRPC server that's available to guests. Every
// WebAssembly module has its own instance of a gRPC server.
type moduleServer struct {
	executor   *Executor
	processID  uuid.UUID
	parentID   *uuid.UUID
	moduleSpec ModuleSpec
	logSpec    *LogSpec
	mailbox    <-chan message
}

// Serve serves using the specified net.Listener.
func (t *moduleServer) Serve(l net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle(serverv1connect.NewTimecraftServiceHandler(
		&grpcServer{instance: t},
		connect.WithCodec(grpc.Codec{}),
	))
	server := &http.Server{
		Addr:    "timecraft",
		Handler: mux,
		// TODO: timeouts/limits
	}
	return server.Serve(l)
}

type grpcServer struct {
	serverv1connect.UnimplementedTimecraftServiceHandler

	instance *moduleServer
}

func (s *grpcServer) Spawn(ctx context.Context, req *connect.Request[v1.SpawnRequest]) (*connect.Response[v1.SpawnResponse], error) {
	moduleSpec := s.instance.moduleSpec // shallow copy
	if req.Msg.Path != "" {
		moduleSpec.Path = req.Msg.Path
	}
	moduleSpec.Args = req.Msg.Args
	moduleSpec.Dials = nil   // not supported
	moduleSpec.Listens = nil // not supported

	var logSpec *LogSpec
	if parentLog := s.instance.logSpec; parentLog != nil {
		logSpec = &LogSpec{
			StartTime:   time.Now(),
			Compression: parentLog.Compression,
			BatchSize:   parentLog.BatchSize,
		}
	}

	childID, err := s.instance.executor.Start(moduleSpec, logSpec, &s.instance.processID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to start module: %w", err))
	}

	res := connect.NewResponse(&v1.SpawnResponse{ProcessId: childID.String()})
	return res, nil
}

func (s *grpcServer) Kill(ctx context.Context, req *connect.Request[v1.KillRequest]) (*connect.Response[v1.KillResponse], error) {
	processID, err := uuid.Parse(req.Msg.ProcessId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid process ID: %w", err))
	}
	switch err := s.instance.executor.Stop(processID, &s.instance.processID); err {
	case nil:
		return connect.NewResponse(&v1.KillResponse{}), nil
	case errNotFound:
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("unknown process ID: %s", processID))
	case errForbidden:
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("cannot kill process %s", processID))
	default:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
}

func (s *grpcServer) Send(ctx context.Context, req *connect.Request[v1.SendRequest]) (*connect.Response[v1.SendResponse], error) {
	processID, err := uuid.Parse(req.Msg.ProcessId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid process ID: %w", err))
	}
	switch err := s.instance.executor.Send(processID, s.instance.processID, req.Msg.Message); err {
	case nil:
		return connect.NewResponse(&v1.SendResponse{}), nil
	case errNotFound:
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("unknown process ID: %s", processID))
	default:
		return nil, connect.NewError(connect.CodeInternal, err)
	}
}

func (s *grpcServer) Receive(ctx context.Context, req *connect.Request[v1.ReceiveRequest]) (*connect.Response[v1.ReceiveResponse], error) {
	msg := <-s.instance.mailbox
	return connect.NewResponse(&v1.ReceiveResponse{SenderId: msg.sender.String(), Message: msg.body}), nil
}

func (s *grpcServer) Parent(ctx context.Context, req *connect.Request[v1.ParentRequest]) (*connect.Response[v1.ParentResponse], error) {
	res := &v1.ParentResponse{}
	if parentID := s.instance.parentID; parentID == nil {
		res.Root = true
	} else {
		res.ParentId = parentID.String()
	}
	return connect.NewResponse(res), nil
}

func (s *grpcServer) Version(context.Context, *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	return connect.NewResponse(&v1.VersionResponse{Version: Version()}), nil
}
