package storage

import (
	"context"

	"github.com/buildbarn/bb-asset-hub/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// AssetStore is a wrapper around a BlobAccess to inteface well with
// AssetReference messages
type AssetStore struct {
	blobAccess              blobstore.BlobAccess
	maximumMessageSizeBytes int
}

// NewAssetStore creates a new AssetStore from a BlobAccess
func NewAssetStore(ba blobstore.BlobAccess, maximumMessageSizeBytes int) *AssetStore {
	return &AssetStore{
		blobAccess:              ba,
		maximumMessageSizeBytes: maximumMessageSizeBytes,
	}
}

// Get a digest given a reference
func (rs *AssetStore) Get(ctx context.Context, ref *asset.AssetReference, instance digest.InstanceName) (*asset.Asset, error) {
	refDigest, err := assetReferenceToDigest(ref, instance)
	if err != nil {
		return nil, err
	}

	data, err := rs.blobAccess.Get(ctx, refDigest).ToProto(
		&asset.Asset{},
		rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	return data.(*asset.Asset), nil
}

// Put a digest into the store referenced by a given reference
func (rs *AssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, instance digest.InstanceName) error {
	refDigest, err := assetReferenceToDigest(ref, instance)
	if err != nil {
		return err
	}
	return rs.blobAccess.Put(ctx, refDigest, buffer.NewProtoBufferFromProto(data, buffer.UserProvided))
}
