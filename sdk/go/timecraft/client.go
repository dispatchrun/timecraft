package timecraft

import (
	"context"
	"io"
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

// TaskInfo is information about a task.
type TaskInfo struct {
	State TaskState
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

// SubmitTask submits an HTTP request to a WebAssembly module.
//
// The task is executed asynchronously. The method returns a TaskID that can be
// used to query the task status and fetch the response when ready.
func (c *Client) SubmitTask(ctx context.Context, module ModuleSpec, r *http.Request) (TaskID, error) {
	var body []byte
	if r.Body != nil {
		var err error
		if body, err = io.ReadAll(r.Body); err != nil {
			return "", err
		}
	}

	headers := make([]*v1.Header, len(r.Header))
	for name, values := range r.Header {
		for _, value := range values {
			headers = append(headers, &v1.Header{Name: name, Value: value})
		}
	}

	req := connect.NewRequest(&v1.SubmitTaskRequest{
		Module: &v1.ModuleSpec{Path: module.Path, Args: module.Args},
		Request: &v1.HTTPRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: headers,
			Body:    body,
		},
	})
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
	return &TaskInfo{
		State: TaskState(res.Msg.State),
	}, nil
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
