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
	moduleSpec := s.moduleSpec // inherit from the parent
	moduleSpec.Dials = nil     // not supported
	moduleSpec.Listens = nil   // not supported
	moduleSpec.Args = req.Msg.Module.Args
	if path := req.Msg.Module.Path; path != "" {
		moduleSpec.Path = path
	}

	var logSpec *LogSpec
	if s.logSpec != nil {
		logSpec = &LogSpec{
			StartTime:   time.Now(),
			Compression: s.logSpec.Compression,
			BatchSize:   s.logSpec.BatchSize,
		}
	}

	r := req.Msg.Request
	httpRequest := HTTPRequest{
		Method:  r.Method,
		Path:    r.Path,
		Body:    r.Body,
		Headers: make(http.Header, len(r.Headers)),
	}
	for _, h := range r.Headers {
		httpRequest.Headers[h.Name] = append(httpRequest.Headers[h.Name], h.Value)
	}

	taskID, err := s.scheduler.SubmitTask(moduleSpec, logSpec, httpRequest)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to submit task: %w", err))
	}
	return connect.NewResponse(&v1.SubmitTaskResponse{TaskId: taskID.String()}), nil
}

func (s *Server) LookupTask(ctx context.Context, req *connect.Request[v1.LookupTaskRequest]) (*connect.Response[v1.LookupTaskResponse], error) {
	taskID, err := uuid.Parse(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid task ID: %w", err))
	}
	task, ok := s.scheduler.Lookup(taskID)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no task with ID %s", taskID))
	}
	res := connect.NewResponse(&v1.LookupTaskResponse{
		State: v1.TaskState(task.state),
	})
	switch task.state {
	case Error:
		res.Msg.ErrorMessage = task.err.Error()
	case Done:
		r := &v1.HTTPResponse{
			StatusCode: int32(task.res.StatusCode),
			Body:       task.res.Body,
			Headers:    make([]*v1.Header, 0, len(task.res.Headers)),
		}
		for name, values := range task.res.Headers {
			for _, value := range values {
				r.Headers = append(r.Headers, &v1.Header{Name: name, Value: value})
			}
		}
		res.Msg.Response = r
	}
	return res, nil
}

func (s *Server) Version(context.Context, *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	return connect.NewResponse(&v1.VersionResponse{Version: Version()}), nil
}
