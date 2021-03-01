package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	protostatus "google.golang.org/genproto/googleapis/rpc/status"

	"google.golang.org/grpc/status"
)

type errorPushServer struct {
	err *protostatus.Status
}

// NewErrorPushServer creates a Remote Asset API Push service which
// simply returns a set gRPC status
func NewErrorPushServer(err *protostatus.Status) remoteasset.PushServer {
	return &errorPushServer{
		err: err,
	}
}

func (ep *errorPushServer) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	return nil, status.ErrorProto(ep.err)
}

func (ep *errorPushServer) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	return nil, status.ErrorProto(ep.err)
}
