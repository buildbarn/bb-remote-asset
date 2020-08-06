package fetch

import (
	"context"

	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cachingFetcher struct {
	fetcher    remoteasset.FetchServer
	assetStore *storage.AssetStore
}

// NewCachingFetcher creates a decorator for remoteasset.FetchServer implementations to avoid having to fetch the
// blob remotely multiple times
func NewCachingFetcher(fetcher remoteasset.FetchServer, refStore *storage.AssetStore) remoteasset.FetchServer {
	return &cachingFetcher{
		fetcher: 	        fetcher,
		assetStore: 		refStore,
	}
}


func (cf *cachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)

	// Check refStore
	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		assetData, err := cf.assetStore.Get(ctx, assetRef, instanceName)
		if err != nil {
			continue
		}

		// Successful retrieval from the asset reference cache
		return &remoteasset.FetchBlobResponse{
			Status: 	status.New(codes.OK, "Blob fetched successfully from asset cache").Proto(),
			Uri: 		uri,
			Qualifiers: req.Qualifiers,
			BlobDigest: assetData.Digest,
		}, nil
	}

	// Cache Miss
	// Fetch from wrapped fetcher
	response, err := cf.fetcher.FetchBlob(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache fetched blob
	assetRef := storage.NewAssetReference(response.Uri, response.Qualifiers)
	assetData := storage.NewAsset(response.BlobDigest)
	err = cf.assetStore.Put(ctx, assetRef, assetData, instanceName)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (cf *cachingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return &remoteasset.FetchDirectoryResponse{
		Status: status.New(codes.Unimplemented, "This feature is not currently supported!").Proto(),
	}, nil
}