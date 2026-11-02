package storage

import (
	"context"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// blobAccessAssetStore is an AssetStore backed by a BlobAccess storing
// Asset messages.
type blobAccessAssetStore struct {
	blobAccess blobstore.BlobAccess[*asset.Asset]
}

// NewBlobAccessAssetStore creates a new AssetStore from a BlobAccess
func NewBlobAccessAssetStore(ba blobstore.BlobAccess[*asset.Asset]) AssetStore {
	return &blobAccessAssetStore{
		blobAccess: ba,
	}
}

// Get an asset given a reference
func (rs *blobAccessAssetStore) Get(ctx context.Context, ref *asset.AssetReference, digestFunction digest.Function) (*asset.Asset, error) {
	_, refDigest, err := SerialiseProto(ref, digestFunction)
	if err != nil {
		return nil, err
	}
	return rs.blobAccess.Get(ctx, refDigest)
}

// Put an asset into the store referenced by a given reference
func (rs *blobAccessAssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, digestFunction digest.Function) error {
	_, refDigest, err := SerialiseProto(ref, digestFunction)
	if err != nil {
		return err
	}
	return rs.blobAccess.Put(ctx, refDigest, data)
}
