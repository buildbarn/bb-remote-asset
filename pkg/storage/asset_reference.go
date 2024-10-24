package storage

import (
	"sort"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
)

// NewAssetReference creates a new AssetReference from a URI and a list
// of qualifiers. Mainly this is a wrapper to ensure the qualifiers get
// sorted
func NewAssetReference(uris []string, qualifiers []*remoteasset.Qualifier) *asset.AssetReference {
	sortedQualifiers := qualifier.Sorter(qualifiers)
	sort.Sort(sortedQualifiers)
	sort.Strings(uris)
	return &asset.AssetReference{Uris: uris, Qualifiers: sortedQualifiers.ToArray()}
}
