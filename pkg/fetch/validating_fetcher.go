package fetch

import (
	"context"
	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validatingFetcher struct {
	fetcher remoteasset.FetchServer
}

// NewAssetFetchServer creates a blank Fetcher with both FetchBlob
// and FetchDirectory unimplemented
func NewValidatingFetcher(fetcher remoteasset.FetchServer) remoteasset.FetchServer {
	return &validatingFetcher{
		fetcher: fetcher,
	}
}

func (vf *validatingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Error(codes.InvalidArgument, "FetchBlob does not support requests without any URIs specified.")
	}
	return vf.fetcher.FetchBlob(ctx, req)
}

func (vf *validatingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Error(codes.InvalidArgument, "FetchDirectory does not support requests without any URIs specified.")
	}
	return vf.fetcher.FetchDirectory(ctx, req)
}
