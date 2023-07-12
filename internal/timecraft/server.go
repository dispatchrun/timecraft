package timecraft

import (
	"context"
	"errors"
	"fmt"
	"math"
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
	ProcessManager *ProcessManager
	Scheduler      *TaskScheduler
}

// NewServer creates a new Server.
func (f *ServerFactory) NewServer(ctx context.Context, processID uuid.UUID, moduleSpec ModuleSpec, logSpec *LogSpec) *Server {
	return &Server{
		ctx:        ctx,
		processes:  f.ProcessManager,
		tasks:      NewTaskGroup(f.Scheduler),
		processID:  processID,
		moduleSpec: moduleSpec,
		logSpec:    logSpec,
	}
}

// Server is a gRPC server that's available to guests. Every
// WebAssembly module has its own instance of a gRPC server.
type Server struct {
	serverv1connect.UnimplementedTimecraftServiceHandler

	ctx        context.Context
	tasks      *TaskGroup
	processes  *ProcessManager
	processID  uuid.UUID
	moduleSpec ModuleSpec
	logSpec    *LogSpec
}

func (s *Server) Close() error {
	return s.tasks.Close()
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

func (s *Server) SubmitTasks(ctx context.Context, req *connect.Request[v1.SubmitTasksRequest]) (*connect.Response[v1.SubmitTasksResponse], error) {
	res := connect.NewResponse(&v1.SubmitTasksResponse{
		TaskId: make([]string, len(req.Msg.Requests)),
	})
	for i, taskRequest := range req.Msg.Requests {
		taskID, err := s.submitTask(taskRequest)
		if err != nil {
			return nil, err
		}
		res.Msg.TaskId[i] = taskID.String()
	}
	return res, nil
}

func (s *Server) submitTask(req *v1.TaskRequest) (TaskID, error) {
	moduleSpec := s.moduleSpec // inherit from the parent
	moduleSpec.Dials = nil     // not supported
	moduleSpec.Listens = nil   // not supported
	moduleSpec.Stdin = nil     // task handlers receive no data on stdin
	moduleSpec.Args = req.Module.Args
	moduleSpec.Env = append(moduleSpec.Env[:len(moduleSpec.Env):len(moduleSpec.Env)], req.Module.Env...)
	if path := req.Module.Path; path != "" {
		moduleSpec.Path = path
	}
	// The task processes can only bind on their virtual network, we don't
	// create bridges to the host network for them. Only timecraft can open
	// connections and send requests to those processes.
	moduleSpec.HostNetworkBinding = false

	var logSpec *LogSpec
	if s.logSpec != nil {
		logSpec = &LogSpec{
			StartTime:   time.Now(),
			Compression: s.logSpec.Compression,
			BatchSize:   s.logSpec.BatchSize,
		}
	}

	var input TaskInput
	switch in := req.Input.(type) {
	case *v1.TaskRequest_HttpRequest:
		httpRequest := &HTTPRequest{
			Method:  in.HttpRequest.Method,
			Path:    in.HttpRequest.Path,
			Body:    in.HttpRequest.Body,
			Headers: make(http.Header, len(in.HttpRequest.Headers)),
			Port:    int(in.HttpRequest.Port),
		}
		for _, h := range in.HttpRequest.Headers {
			httpRequest.Headers[h.Name] = append(httpRequest.Headers[h.Name], h.Value)
		}
		input = httpRequest
	}

	taskID, err := s.tasks.Submit(moduleSpec, logSpec, input, s.processID)
	if err != nil {
		return TaskID{}, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to submit task: %w", err))
	}
	return taskID, nil
}

func (s *Server) LookupTasks(ctx context.Context, req *connect.Request[v1.LookupTasksRequest]) (*connect.Response[v1.LookupTasksResponse], error) {
	res := connect.NewResponse(&v1.LookupTasksResponse{
		Responses: make([]*v1.TaskResponse, len(req.Msg.TaskId)),
	})
	for i, rawTaskID := range req.Msg.TaskId {
		taskID, err := uuid.Parse(rawTaskID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid task ID: %w", err))
		}
		res.Msg.Responses[i] = s.lookupTask(taskID)
	}
	return res, nil
}

func (s *Server) lookupTask(taskID TaskID) (res *v1.TaskResponse) {
	res = &v1.TaskResponse{
		TaskId: taskID.String(),
	}
	task, ok := s.tasks.Lookup(taskID)
	if !ok {
		res.State = v1.TaskState_TASK_STATE_ERROR
		res.ErrorMessage = "task not found"
		return
	}
	res.State = v1.TaskState(task.state)
	if task.processID != (ProcessID{}) {
		res.ProcessId = task.processID.String()
	}
	if task.err != nil {
		res.ErrorMessage = task.err.Error()
	}
	switch output := task.output.(type) {
	case *HTTPResponse:
		httpResponse := &v1.HTTPResponse{
			StatusCode: int32(output.StatusCode),
			Body:       output.Body,
			Headers:    make([]*v1.Header, 0, len(output.Headers)),
		}
		for name, values := range output.Headers {
			for _, value := range values {
				httpResponse.Headers = append(httpResponse.Headers, &v1.Header{Name: name, Value: value})
			}
		}
		res.Output = &v1.TaskResponse_HttpResponse{HttpResponse: httpResponse}
	}
	return
}

func (s *Server) PollTasks(ctx context.Context, req *connect.Request[v1.PollTasksRequest]) (*connect.Response[v1.PollTasksResponse], error) {
	if req.Msg.BatchSize <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("batch size must be > 0"))
	}

	delay := time.Duration(req.Msg.TimeoutNs)
	if delay < 0 {
		delay = math.MaxInt64
	}
	timeout := time.After(delay)

	res := connect.NewResponse(&v1.PollTasksResponse{})

poll_loop:
	for {
		select {
		case <-ctx.Done():
			break poll_loop
		case <-s.ctx.Done():
			break poll_loop
		case <-timeout:
			break poll_loop
		case taskID := <-s.tasks.Poll():
			taskResponse := s.lookupTask(taskID)
			res.Msg.Responses = append(res.Msg.Responses, taskResponse)
			if len(res.Msg.Responses) == int(req.Msg.BatchSize) {
				break poll_loop
			}
		}
	}
	return res, nil
}

func (s *Server) DiscardTasks(ctx context.Context, req *connect.Request[v1.DiscardTasksRequest]) (*connect.Response[v1.DiscardTasksResponse], error) {
	for i, rawTaskID := range req.Msg.TaskId {
		taskID, err := uuid.Parse(rawTaskID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task ID at index %d is invalid: %w", i, err))
		}
		_ = s.tasks.Discard(taskID)
	}
	return connect.NewResponse(&v1.DiscardTasksResponse{}), nil
}

func (s *Server) ProcessID(context.Context, *connect.Request[v1.ProcessIDRequest]) (*connect.Response[v1.ProcessIDResponse], error) {
	return connect.NewResponse(&v1.ProcessIDResponse{ProcessId: s.processID.String()}), nil
}

func (s *Server) Spawn(ctx context.Context, req *connect.Request[v1.SpawnRequest]) (*connect.Response[v1.SpawnResponse], error) {
	moduleSpec := s.moduleSpec // inherit from the parent
	moduleSpec.Dials = nil     // not supported
	moduleSpec.Listens = nil   // not supported
	moduleSpec.Stdin = nil     // task handlers receive no data on stdin
	moduleSpec.Args = req.Msg.Module.Args
	moduleSpec.Env = append(moduleSpec.Env[:len(moduleSpec.Env):len(moduleSpec.Env)], req.Msg.Module.Env...)
	if path := req.Msg.Module.Path; path != "" {
		moduleSpec.Path = path
	}
	// The task processes can only bind on their virtual network, we don't
	// create bridges to the host network for them. Only timecraft can open
	// connections and send requests to those processes.
	moduleSpec.HostNetworkBinding = false

	var logSpec *LogSpec
	if s.logSpec != nil {
		logSpec = &LogSpec{
			StartTime:   time.Now(),
			Compression: s.logSpec.Compression,
			BatchSize:   s.logSpec.BatchSize,
		}
	}

	processID, err := s.processes.Start(moduleSpec, logSpec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to spawn process: %w", err))
	}
	processInfo, _ := s.processes.Lookup(processID)
	return connect.NewResponse(&v1.SpawnResponse{
		ProcessId: processID.String(),
		IpAddress: processInfo.Addr.String(),
	}), nil
}

func (s *Server) Version(context.Context, *connect.Request[v1.VersionRequest]) (*connect.Response[v1.VersionResponse], error) {
	return connect.NewResponse(&v1.VersionResponse{Version: Version()}), nil
}
