package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	protostatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/status"
)

type errorPusher struct {
	err *protostatus.Status
}

// NewErrorPusher creates a new Push Server which returns a specified error to all requests.
func NewErrorPusher(err *protostatus.Status) remoteasset.PushServer {
	return &errorPusher{
		err: err,
	}
}

func (ep *errorPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	return nil, status.ErrorProto(ep.err)
}

func (ep *errorPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	return nil, status.ErrorProto(ep.err)
}
