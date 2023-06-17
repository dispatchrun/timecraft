package timecraft

import (
	"context"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
	"golang.org/x/net/http2"
)

// NewClient creates a timecraft client.
func NewClient() (*Client, error) {
	grpcClient := serverv1connect.NewTimecraftServiceClient(httpClient, "http://timecraft/", connect.WithCodec(grpc.Codec{}))

	return &Client{
		grpcClient: grpcClient,
	}, nil
}

var httpClient = &http.Client{
	Transport: &http2.Transport{
		AllowHTTP:      true,
		DialTLSContext: dialContext,
		// TODO: timeouts/limits
	},
}

// Client is a timecraft client.
type Client struct {
	grpcClient serverv1connect.TimecraftServiceClient
}

// Parent returns the ID of the process that started this process.
//
// The boolean flag is true if the process was spawned by another,
// and false if the process is the root process.
func (c *Client) Parent(ctx context.Context) (parentID string, hasParent bool, err error) {
	req := connect.NewRequest(&v1.ParentRequest{})
	res, err := c.grpcClient.Parent(ctx, req)
	if err != nil {
		return "", false, err
	}
	return res.Msg.ParentId, !res.Msg.Root, nil
}

// Spawn spawns a process and returns its ID.
func (c *Client) Spawn(ctx context.Context, path string, args []string) (processID string, err error) {
	req := connect.NewRequest(&v1.SpawnRequest{Path: path, Args: args})
	res, err := c.grpcClient.Spawn(ctx, req)
	if err != nil {
		return "", err
	}
	return res.Msg.ProcessId, nil
}

// Kill kills a process that was spawned by this process.
func (c *Client) Kill(ctx context.Context, processID string) error {
	req := connect.NewRequest(&v1.KillRequest{ProcessId: processID})
	_, err := c.grpcClient.Kill(ctx, req)
	return err
}

// Send sends a message to a process.
func (c *Client) Send(ctx context.Context, processID string, message []byte) error {
	req := connect.NewRequest(&v1.SendRequest{ProcessId: processID, Message: message})
	_, err := c.grpcClient.Send(ctx, req)
	return err
}

// Receive receives a message.
func (c *Client) Receive(ctx context.Context) (processID string, message []byte, err error) {
	req := connect.NewRequest(&v1.ReceiveRequest{})
	res, err := c.grpcClient.Receive(ctx, req)
	if err != nil {
		return "", nil, err
	}
	return res.Msg.SenderId, res.Msg.Message, nil
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
