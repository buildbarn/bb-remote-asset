package storage

import (
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewAsset creates a new Asset from request data.
func NewAsset(digest *remoteexecution.Digest, assetType asset.Asset_AssetType, expireAt *timestamppb.Timestamp) *asset.Asset {
	return &asset.Asset{
		Digest:      digest,
		ExpireAt:    expireAt,
		LastUpdated: timestamppb.Now(),
		Type:        assetType,
	}
}

// NewBlobAsset creates a new Asset (type Blob) from request data.
func NewBlobAsset(digest *remoteexecution.Digest, expireAt *timestamppb.Timestamp) *asset.Asset {
	return NewAsset(digest, asset.Asset_BLOB, expireAt)
}

// NewDirectoryAsset creates a new Asset (type Directory) from request data.
func NewDirectoryAsset(digest *remoteexecution.Digest, expireAt *timestamppb.Timestamp) *asset.Asset {
	return NewAsset(digest, asset.Asset_DIRECTORY, expireAt)
}
