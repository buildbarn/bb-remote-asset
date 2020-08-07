package storage

import (
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-asset-hub/pkg/proto/asset"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// NewAsset creates a new Asset from request data.
func NewAsset(digest *remoteexecution.Digest, expireAt *timestamp.Timestamp) *asset.Asset {
	return &asset.Asset{
		Digest:      digest,
		ExpireAt:    expireAt,
		LastUpdated: ptypes.TimestampNow(),
	}
}
