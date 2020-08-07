package fetch

import (
	"log"
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
)

type loggingFetcher struct {
	fetcher remoteasset.FetchServer
}

// NewLoggingFetcher creates a fetcher which logs requests and results
func NewLoggingFetcher(fetcher remoteasset.FetchServer) remoteasset.FetchServer {
	return &loggingFetcher{
		fetcher: fetcher,
	}
}

func (lf *loggingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	log.Printf("Fetching Blob %s with qualifiers %s", req.Uris, req.Qualifiers)
	resp, err := lf.fetcher.FetchBlob(ctx, req)
	log.Printf("FetchBlob completed for %s with status code %d", req.Uris, resp.Status.GetCode())
	return resp, err
}

func (lf *loggingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	log.Printf("Fetching Directory %s with qualifiers %s", req.Uris, req.Qualifiers)
	resp, err := lf.fetcher.FetchDirectory(ctx, req)
	log.Printf("> FetchDirectory completed for %s with status code %d", req.Uris, resp.Status.GetCode())
	return resp, err
}
