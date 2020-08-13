package fetch

import (
	"context"
	"log"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/qualifier"
	"google.golang.org/grpc/status"
)

type loggingFetcher struct {
	fetcher Fetcher
}

// NewLoggingFetcher creates a fetcher which logs requests and results
func NewLoggingFetcher(fetcher Fetcher) Fetcher {
	return &loggingFetcher{
		fetcher: fetcher,
	}
}

func (lf *loggingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	log.Printf("Fetching Blob %s with qualifiers %s", req.Uris, req.Qualifiers)
	resp, err := lf.fetcher.FetchBlob(ctx, req)
	if err == nil {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, resp.Status.GetCode())
	} else {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, status.Code(err))
	}
	return resp, err
}

func (lf *loggingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	log.Printf("Fetching Directory %s with qualifiers %s", req.Uris, req.Qualifiers)
	resp, err := lf.fetcher.FetchDirectory(ctx, req)
	if err == nil {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, resp.Status.GetCode())
	} else {
		log.Printf("FetchBlob completed for %s with status code %d", req.Uris, status.Code(err))
	}
	return resp, err
}

func (lf *loggingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return lf.fetcher.CheckQualifiers(qualifiers)
}
