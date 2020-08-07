package fetch

import (
	"context"
	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type notFoundFetcher struct {
}

// NewNotFoundFetcher creates a blank Fetcher with both FetchBlob
// and FetchDirectory return NotFound status codes.
func NewNotFoundFetcher() remoteasset.FetchServer {
	return &notFoundFetcher{
	}
}

func (nf *notFoundFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	return &remoteasset.FetchBlobResponse{
		Status: status.New(codes.NotFound, "Blob could not be found at any of the provided URIs").Proto(),
	}, nil
}

func (nf *notFoundFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return &remoteasset.FetchDirectoryResponse{
		Status: status.New(codes.NotFound, "Directory could not be found at any of the provided URIs").Proto(),
	}, nil
}
