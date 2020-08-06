package blobstore

import (
	"io"

	"github.com/buildbarn/bb-asset-hub/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
)

type assetStorageType struct{}

func (f assetStorageType) GetDigestKey(blobDigest digest.Digest) string {
	return blobDigest.GetKey(digest.KeyWithInstance)
}

func (f assetStorageType) NewBufferFromByteSlice(digest digest.Digest, data []byte, repairStrategy buffer.RepairStrategy) buffer.Buffer {
	return buffer.NewProtoBufferFromByteSlice(&asset.Asset{}, data, repairStrategy)
}

func (f assetStorageType) NewBufferFromReader(digest digest.Digest, r io.ReadCloser, repairStrategy buffer.RepairStrategy) buffer.Buffer {
	return buffer.NewProtoBufferFromReader(&asset.Asset{}, r, repairStrategy)
}

// AssetStorageType is capable of creating identifiers and buffers for
// objects stored in the Asset Store
var AssetStorageType blobstore.StorageType = assetStorageType{}
