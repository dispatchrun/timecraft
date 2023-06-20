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

// Version fetches the timecraft version.
func (c *Client) Version(ctx context.Context) (string, error) {
	req := connect.NewRequest(&v1.VersionRequest{})
	res, err := c.grpcClient.Version(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Msg.Version, nil
}
