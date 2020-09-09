package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"
)

type localCachingPusher struct {
	pusher     remoteasset.PushServer
	assetStore *storage.AssetStore
}

// NewLocalCachingPusher creates a gRPC service for serving the contents
// of a Remote Asset Push server.
func NewLocalCachingPusher(pusher remoteasset.PushServer, assetStore *storage.AssetStore) remoteasset.PushServer {
	return &localCachingPusher{
		pusher:     pusher,
		assetStore: assetStore,
	}
}

func (lcp *localCachingPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		assetData := storage.NewAsset(req.BlobDigest, req.ExpireAt)
		err = lcp.assetStore.Put(ctx, assetRef, assetData, instanceName)
		if err != nil {
			return nil, err
		}
	}

	return lcp.pusher.PushBlob(ctx, req)
}

func (lcp *localCachingPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	instanceName, err := bb_digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		assetData := storage.NewAsset(req.RootDirectoryDigest, req.ExpireAt)
		err := lcp.assetStore.Put(ctx, assetRef, assetData, instanceName)
		if err != nil {
			return nil, err
		}
	}

	return lcp.pusher.PushDirectory(ctx, req)
}
