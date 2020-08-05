package storage

import (
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-asset-hub/pkg/proto/asset"
)

// NewAsset creates a new Asset from request data.
func NewAsset(digest *remoteexecution.Digest) *asset.Asset {
	return &asset.Asset{
		Digest: digest,
	}
}
