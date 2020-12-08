package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/protobuf/proto"
)

// NewAssetReference creates a new AssetReference from a URI and a list
// of qualifiers. Mainly this is a wrapper to ensure the qualifiers get
// sorted
func NewAssetReference(uri string, qualifiers []*remoteasset.Qualifier) *asset.AssetReference {
	sortedQualifiers := qualifier.Sorter(qualifiers)
	sort.Sort(sortedQualifiers)
	return &asset.AssetReference{Uri: uri, Qualifiers: sortedQualifiers.ToArray()}
}

// AssetReferenceToDigest converts an AssetReference into a bb-storage Digest of its
// wire format
func AssetReferenceToDigest(ar *asset.AssetReference, instance digest.InstanceName) (digest.Digest, error) {
	wireFormat, err := proto.Marshal(ar)
	if err != nil {
		return digest.Digest{}, err
	}

	hash := sha256.Sum256(wireFormat)
	sizeBytes := int64(len(wireFormat))

	return instance.NewDigest(hex.EncodeToString(hash[:]), sizeBytes)
}
