package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"google.golang.org/genproto/googleapis/rpc/status"
)

type errorFetcher struct {
	err *status.Status
}

// NewErrorFetcher creates a Remote Asset API Fetch service which simply returns a
// set gRPC status
func NewErrorFetcher(err *status.Status) remoteasset.FetchServer {
	return &errorFetcher{
		err: err,
	}
}

func (ef *errorFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	return &remoteasset.FetchBlobResponse{
		Status: ef.err,
	}, nil
}

func (ef *errorFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return &remoteasset.FetchDirectoryResponse{
		Status: ef.err,
	}, nil
}
