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
func (c *Client) Parent() (string, bool, error) {
	req := connect.NewRequest(&v1.ParentRequest{})
	res, err := c.grpcClient.Parent(context.Background(), req)
	if err != nil {
		return "", false, err
	}
	return res.Msg.ParentId, !res.Msg.Root, nil
}

// Spawn spawns a process and returns its ID.
func (c *Client) Spawn(path string, args []string) (string, error) {
	req := connect.NewRequest(&v1.SpawnRequest{Path: path, Args: args})
	res, err := c.grpcClient.Spawn(context.Background(), req)
	if err != nil {
		return "", err
	}
	return res.Msg.ProcessId, nil
}

// Version fetches the timecraft version.
func (c *Client) Version() (string, error) {
	req := connect.NewRequest(&v1.VersionRequest{})
	res, err := c.grpcClient.Version(context.Background(), req)
	if err != nil {
		return "", err
	}
	return res.Msg.Version, nil
}
