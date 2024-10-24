package storage

import (
	"context"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// blobAccessAssetStore is an AssetStore backed by a blobAccess.
type blobAccessAssetStore struct {
	blobAccess              blobstore.BlobAccess
	maximumMessageSizeBytes int
}

// NewBlobAccessAssetStore creates a new AssetStore from a BlobAccess
func NewBlobAccessAssetStore(ba blobstore.BlobAccess, maximumMessageSizeBytes int) AssetStore {
	return &blobAccessAssetStore{
		blobAccess:              ba,
		maximumMessageSizeBytes: maximumMessageSizeBytes,
	}
}

// Get a digest given a reference
func (rs *blobAccessAssetStore) Get(ctx context.Context, ref *asset.AssetReference, instance digest.InstanceName) (*asset.Asset, error) {
	refDigest, err := ProtoToDigest(ref, instance)
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
func (rs *blobAccessAssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, instance digest.InstanceName) error {
	refDigest, err := ProtoToDigest(ref, instance)
	if err != nil {
		return err
	}
	return rs.blobAccess.Put(ctx, refDigest, buffer.NewProtoBufferFromProto(data, buffer.UserProvided))
}
