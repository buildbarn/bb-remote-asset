package fetch

import (
	"context"
	"time"

	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"

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
func NewCachingFetcher(fetcher remoteasset.FetchServer, assetStore *storage.AssetStore) remoteasset.FetchServer {
	return &cachingFetcher{
		fetcher:    fetcher,
		assetStore: assetStore,
	}
}

func (cf *cachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	var oldestContentAccepted time.Time = time.Unix(0, 0)
	if req.OldestContentAccepted != nil {
		oldestContentAccepted, err = ptypes.Timestamp(req.OldestContentAccepted)
		if err != nil {
			return nil, err
		}
	}

	// Check assetStore
	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		assetData, err := cf.assetStore.Get(ctx, assetRef, instanceName)
		if err != nil {
			continue
		}

		// Check whether the asset has expired, making sure ExpireAt was set
		if assetData.ExpireAt != nil {
			expireTime, err := ptypes.Timestamp(assetData.ExpireAt)
			if err != nil || (expireTime.Before(time.Now()) && !expireTime.Equal(time.Unix(0, 0))) {
				continue
			}
		}

		// Check that content is newer than the oldest accepted by the request
		if oldestContentAccepted != time.Unix(0, 0) {
			updateTime, err := ptypes.Timestamp(assetData.LastUpdated)
			if err != nil || updateTime.Before(oldestContentAccepted) {
				continue
			}
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
		return nil, err
	}

	// Cache fetched blob
	assetRef := storage.NewAssetReference(response.Uri, response.Qualifiers)
	assetData := storage.NewAsset(response.BlobDigest, getDefaultTimestamp())
	err = cf.assetStore.Put(ctx, assetRef, assetData, instanceName)
	if err != nil {
		return response, err
	}

	return response, nil
}

func (cf *cachingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}

	var oldestContentAccepted time.Time = time.Unix(0, 0)
	if req.OldestContentAccepted != nil {
		oldestContentAccepted, err = ptypes.Timestamp(req.OldestContentAccepted)
		if err != nil {
			return nil, err
		}
	}

	// Check refStore
	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		assetData, err := cf.assetStore.Get(ctx, assetRef, instanceName)
		if err != nil {
			continue
		}

		// Check whether the asset has expired, making sure ExpireAt was set
		if assetData.ExpireAt != nil {
			expireTime, err := ptypes.Timestamp(assetData.ExpireAt)
			if err != nil || (expireTime.Before(time.Now()) && !expireTime.Equal(time.Unix(0, 0))) {
				continue
			}
		}

		// Check that content is newer than the oldest accepted by the request
		if oldestContentAccepted != time.Unix(0, 0) {
			updateTime, err := ptypes.Timestamp(assetData.LastUpdated)
			if err != nil || updateTime.Before(oldestContentAccepted) {
				continue
			}
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
		return nil, err
	}

	// Cache fetched blob
	assetRef := storage.NewAssetReference(response.Uri, response.Qualifiers)
	assetData := storage.NewAsset(response.RootDirectoryDigest, getDefaultTimestamp())
	err = cf.assetStore.Put(ctx, assetRef, assetData, instanceName)
	if err != nil {
		return response, err
	}

	return response, nil
}

func getDefaultTimestamp() *timestamp.Timestamp {
	ts, _ := ptypes.TimestampProto(time.Unix(0, 0))
	return ts
}
