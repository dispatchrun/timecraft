package client

import (
	"context"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
	"golang.org/x/net/http2"
)

// Socket is the socket that timecraft guests connect to in order to
// interact with the timecraft server on the host. Note that this is a
// virtual socket.
const Socket = "timecraft.sock"

var httpClient = &http.Client{
	Transport: &http2.Transport{
		AllowHTTP:      true,
		DialTLSContext: dialContext,
		// TODO: timeouts/limits
	},
}

// NewClient creates a client to the timecraft server.
func NewClient() (*Client, error) {
	grpcClient := serverv1connect.NewTimecraftServiceClient(httpClient, "http://timecraft/", connect.WithCodec(grpc.Codec{}))

	return &Client{
		grpcClient: grpcClient,
	}, nil
}

// Client is a client to server.TimecraftServer.
type Client struct {
	grpcClient serverv1connect.TimecraftServiceClient
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
