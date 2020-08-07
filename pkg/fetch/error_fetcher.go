package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	protostatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/status"
)

type errorFetcher struct {
	err *protostatus.Status
}

// NewErrorFetcher creates a Remote Asset API Fetch service which simply returns a
// set gRPC status
func NewErrorFetcher(err *protostatus.Status) remoteasset.FetchServer {
	return &errorFetcher{
		err: err,
	}
}

func (ef *errorFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	return nil, status.ErrorProto(ef.err)
}

func (ef *errorFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return nil, status.ErrorProto(ef.err)
}
