package timecraft

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bufbuild/connect-go"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
)

// NewClient creates a timecraft client.
func NewClient() (*Client, error) {
	grpcClient := serverv1connect.NewTimecraftServiceClient(
		httpClient,
		"http://timecraft/",
		connect.WithAcceptCompression("gzip", nil, nil), // disable gzip for now
		connect.WithProtoJSON(),                         // use JSON for now
		// connect.WithCodec(grpc.Codec{}),
	)

	return &Client{
		grpcClient: grpcClient,
	}, nil
}

var httpClient = &http.Client{
	Transport: &http.Transport{
		DialContext: dialContext,
		// TODO: timeouts/limits
	},
}

// Client is a timecraft client.
type Client struct {
	grpcClient serverv1connect.TimecraftServiceClient
}

// ModuleSpec is a WebAssembly module specification.
type ModuleSpec struct {
	Path string
	Args []string
}

// TaskID is a task identifier.
type TaskID string

// ProcessID is a process identifier.
type ProcessID string

// TaskInfo is information about a task.
type TaskInfo struct {
	// State is the current state of the task.
	State TaskState

	// Error is the error that occurred during task initialization or execution
	// (if applicable).
	Error error

	// Output is the output of the task, if it executed successfully.
	Output TaskOutput

	// ProcessID is the identifier of the process that handled the task
	// (if applicable).
	ProcessID ProcessID
}

// TaskState is the state of a task.
type TaskState int

const (
	// Queued indicates that the task is waiting to be scheduled.
	Queued TaskState = iota + 1

	// Initializing indicates that the task is in the process of being scheduled
	// on to a process.
	Initializing

	// Executing indicates that the task is currently being executed.
	Executing

	// Error indicates that the task failed with an error. This is a terminal
	// status.
	Error

	// Done indicates that the task executed successfully. This is a terminal
	// status.
	Done
)

// TaskInput is input to a task.
type TaskInput interface{ taskInput() }

// HTTPRequest is an HTTP request.
type HTTPRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

func (*HTTPRequest) taskInput() {}

// TaskOutput is output from a task.
type TaskOutput interface{ taskOutput() }

// HTTPResponse is an HTTP response.
type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (*HTTPResponse) taskOutput() {}

// SubmitTask submits an HTTP request to a WebAssembly module.
//
// The task is executed asynchronously. The method returns a TaskID that can be
// used to query the task status and fetch the response when ready.
func (c *Client) SubmitTask(ctx context.Context, module ModuleSpec, input TaskInput) (TaskID, error) {
	req := connect.NewRequest(&v1.SubmitTaskRequest{
		Module: &v1.ModuleSpec{Path: module.Path, Args: module.Args},
	})

	switch in := input.(type) {
	case *HTTPRequest:
		headers := make([]*v1.Header, 0, len(in.Headers))
		for name, values := range in.Headers {
			for _, value := range values {
				headers = append(headers, &v1.Header{Name: name, Value: value})
			}
		}
		req.Msg.Input = &v1.SubmitTaskRequest_HttpRequest{HttpRequest: &v1.HTTPRequest{
			Method:  in.Method,
			Path:    in.Path,
			Body:    in.Body,
			Headers: headers,
		}}
	default:
		return "", fmt.Errorf("invalid task input: %v", input)
	}
	res, err := c.grpcClient.SubmitTask(ctx, req)
	if err != nil {
		return "", err
	}
	return TaskID(res.Msg.TaskId), nil
}

// LookupTask retrieves task information.
func (c *Client) LookupTask(ctx context.Context, taskID TaskID) (*TaskInfo, error) {
	req := connect.NewRequest(&v1.LookupTaskRequest{TaskId: string(taskID)})
	res, err := c.grpcClient.LookupTask(ctx, req)
	if err != nil {
		return nil, err
	}
	task := &TaskInfo{
		State:     TaskState(res.Msg.State),
		ProcessID: ProcessID(res.Msg.ProcessId),
	}
	if task.State == Error {
		task.Error = errors.New(res.Msg.ErrorMessage)
	}
	switch out := res.Msg.Output.(type) {
	case *v1.LookupTaskResponse_HttpResponse:
		httpResponse := &HTTPResponse{
			StatusCode: int(out.HttpResponse.StatusCode),
			Body:       out.HttpResponse.Body,
			Headers:    make(http.Header, len(out.HttpResponse.Headers)),
		}
		for _, h := range out.HttpResponse.Headers {
			httpResponse.Headers[h.Name] = append(httpResponse.Headers[h.Name], h.Value)
		}
		task.Output = httpResponse
	}
	return task, nil
}

// Version fetches the timecraft version.
func (c *Client) Version(ctx context.Context) (string, error) {
	req := connect.NewRequest(&v1.VersionRequest{})
	res, err := c.grpcClient.Version(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Msg.Version, nil
}
