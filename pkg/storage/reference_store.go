package storage

import (
	"context"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-asset-hub/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

// ReferenceStore is a wrapper around a BlobAccess to inteface well with
// AssetReference messages
type ReferenceStore struct {
	blobAccess              blobstore.BlobAccess
	maximumMessageSizeBytes int
}

// NewReferenceStore creates a new ReferenceStore from a BlobAccess
func NewReferenceStore(ba blobstore.BlobAccess, maximumMessageSizeBytes int) *ReferenceStore {
	return &ReferenceStore{
		blobAccess:              ba,
		maximumMessageSizeBytes: maximumMessageSizeBytes,
	}
}

// Get a digest given a reference
func (rs *ReferenceStore) Get(ctx context.Context, ref *asset.AssetReference, instance digest.InstanceName) (*remoteexecution.Digest, error) {
	refDigest, err := assetReferenceToDigest(ref, instance)
	if err != nil {
		return nil, err
	}

	digest, err := rs.blobAccess.Get(ctx, refDigest).ToProto(
		&remoteexecution.Digest{},
		rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	return digest.(*remoteexecution.Digest), nil
}

// Put a digest into the store referenced by a given reference
func (rs *ReferenceStore) Put(ctx context.Context, ref *asset.AssetReference, digest *remoteexecution.Digest, instance digest.InstanceName) error {
	refDigest, err := assetReferenceToDigest(ref, instance)
	if err != nil {
		return err
	}
	return rs.blobAccess.Put(ctx, refDigest, buffer.NewProtoBufferFromProto(digest, buffer.UserProvided))
}
