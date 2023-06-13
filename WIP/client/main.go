package main

import (
	"context"
	"log"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
)

func main() {
	client := serverv1connect.NewTimecraftServiceClient(
		http.DefaultClient,
		"http://localhost:8080/",
		connect.WithCodec(grpc.Codec{}),
	)
	req := connect.NewRequest(&v1.SubmitTaskRequest{
		Name: "foobar",
	})
	res, err := client.SubmitTask(context.Background(), req)
	if err != nil {
		panic(err)
	}
	log.Println(res.Msg.Code)
}
