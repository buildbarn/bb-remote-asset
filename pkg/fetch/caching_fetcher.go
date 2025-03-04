package fetch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"google.golang.org/protobuf/types/known/timestamppb"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type cachingFetcher struct {
	fetcher    Fetcher
	assetStore storage.AssetStore
}

// NewCachingFetcher creates a decorator for remoteasset.FetchServer implementations to avoid having to fetch the
// blob remotely multiple times
func NewCachingFetcher(fetcher Fetcher, assetStore storage.AssetStore) Fetcher {
	return &cachingFetcher{
		fetcher:    fetcher,
		assetStore: assetStore,
	}
}

func (cf *cachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName)
	if err != nil {
		return nil, err
	}

	var oldestContentAccepted time.Time = time.Unix(0, 0)
	if req.OldestContentAccepted != nil {
		if err := req.OldestContentAccepted.CheckValid(); err != nil {
			return nil, err
		}
		oldestContentAccepted = req.OldestContentAccepted.AsTime()
	}

	allCachingErrors := []error{}

	// Check assetStore
	for _, uri := range req.Uris {
		assetData, err := getAndCheckAsset(ctx, cf.assetStore, uri, req.Qualifiers, digestFunction, oldestContentAccepted)
		if err != nil {
			allCachingErrors = append(allCachingErrors, err)
			continue
		}

		// Successful retrieval from the asset reference cache
		return &remoteasset.FetchBlobResponse{
			Status:     status.New(codes.OK, "Blob fetched successfully from asset cache").Proto(),
			Uri:        uri,
			Qualifiers: req.Qualifiers,
			BlobDigest: assetData.Digest,
		}, nil
	}

	// Cache Miss
	// Fetch from wrapped fetcher
	response, err := cf.fetcher.FetchBlob(ctx, req)
	if err != nil {
		errAsStatus := status.Convert(err)
		return nil, status.Errorf(
			errAsStatus.Code(),
			"%s (retrieving cached blob failed with: %v)",
			errAsStatus.Message(),
			errors.Join(allCachingErrors...),
		)
	}
	if response.Status.Code != 0 {
		return response, nil
	}

	// Cache fetched blob with single URI
	assetRef := storage.NewAssetReference([]string{response.Uri}, response.Qualifiers)
	assetData := storage.NewBlobAsset(response.BlobDigest, getDefaultTimestamp())
	err = cf.assetStore.Put(ctx, assetRef, assetData, digestFunction)
	if err != nil {
		return response, err
	}
	if len(req.Uris) > 1 {
		// Cache fetched blob with list of URIs
		assetRef = storage.NewAssetReference(req.Uris, assetRef.Qualifiers)
		err = cf.assetStore.Put(ctx, assetRef, assetData, digestFunction)
		if err != nil {
			return response, err
		}
	}

	return response, nil
}

func getAndCheckAsset(
	ctx context.Context,
	assetStore storage.AssetStore,
	uri string,
	qualifiers []*remoteasset.Qualifier,
	digestFunction digest.Function,
	oldestContentAccepted time.Time,
) (*asset.Asset, error) {
	assetRef := storage.NewAssetReference([]string{uri}, qualifiers)
	assetData, err := assetStore.Get(ctx, assetRef, digestFunction)
	if err != nil {
		return nil, err
	}

	// Check whether the asset has expired, making sure ExpireAt was set
	if assetData.ExpireAt != nil {
		expireTime := assetData.ExpireAt.AsTime()
		if expireTime.Before(time.Now()) && !expireTime.Equal(time.Unix(0, 0)) {
			return nil, fmt.Errorf("Asset expired at %v", expireTime)
		}
	}

	// Check that content is newer than the oldest accepted by the request
	if oldestContentAccepted != time.Unix(0, 0) {
		updateTime := assetData.LastUpdated.AsTime()
		if updateTime.Before(oldestContentAccepted) {
			return nil, fmt.Errorf("Asset older than %v", oldestContentAccepted)
		}
	}

	return assetData, nil
}

func (cf *cachingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName)
	if err != nil {
		return nil, err
	}

	oldestContentAccepted := time.Unix(0, 0)
	if req.OldestContentAccepted != nil {
		oldestContentAccepted = req.OldestContentAccepted.AsTime()
	}

	allCachingErrors := []error{}

	// Check refStore
	for _, uri := range req.Uris {
		assetData, err := getAndCheckAsset(ctx, cf.assetStore, uri, req.Qualifiers, digestFunction, oldestContentAccepted)
		if err != nil {
			allCachingErrors = append(allCachingErrors, err)
			continue
		}

		// Successful retrieval from the asset reference cache
		return &remoteasset.FetchDirectoryResponse{
			Status:              status.New(codes.OK, "Directory fetched successfully from asset cache").Proto(),
			Uri:                 uri,
			Qualifiers:          req.Qualifiers,
			RootDirectoryDigest: assetData.Digest,
		}, nil
	}

	// Cache Miss
	// Fetch from wrapped fetcher
	response, err := cf.fetcher.FetchDirectory(ctx, req)
	if err != nil {
		errAsStatus := status.Convert(err)
		return nil, status.Errorf(
			errAsStatus.Code(),
			"%s (retrieving cached directory failed with: %v)",
			errAsStatus.Message(),
			errors.Join(allCachingErrors...),
		)
	}

	// Cache fetched blob with single URI
	assetRef := storage.NewAssetReference([]string{response.Uri}, response.Qualifiers)
	assetData := storage.NewDirectoryAsset(response.RootDirectoryDigest, getDefaultTimestamp())
	err = cf.assetStore.Put(ctx, assetRef, assetData, digestFunction)
	if err != nil {
		return response, err
	}
	if len(req.Uris) > 1 {
		// Cache fetched blob with list of URIs
		assetRef = storage.NewAssetReference(req.Uris, assetRef.Qualifiers)
		err = cf.assetStore.Put(ctx, assetRef, assetData, digestFunction)
		if err != nil {
			return response, err
		}
	}

	return response, nil
}

func (cf *cachingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return cf.fetcher.CheckQualifiers(qualifiers)
}

func getDefaultTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(0, 0))
}
