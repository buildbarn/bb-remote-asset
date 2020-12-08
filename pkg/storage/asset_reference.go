package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/protobuf/proto"
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

func assetReferenceToAction(ar *asset.AssetReference, directoryDigest *remoteexecution.Digest) (*remoteexecution.Action, *remoteexecution.Command, error) {
	command := &remoteexecution.Command{
		Arguments:   ar.Uris,
		OutputPaths: []string{"out"},
	}
	commandDigest, err := ProtoToDigest(command)
	if err != nil {
		return nil, nil, err
	}
	action := &remoteexecution.Action{
		CommandDigest:   commandDigest,
		InputRootDigest: directoryDigest,
	}
	return action, command, nil
}

func assetReferenceToActionResult(ar *asset.AssetReference, directory bool) (*remoteexecution.ActionResult, error) {
	return &remoteexecution.ActionResult{}, nil
}

// ProtoToDigest converts an arbitrary proto to a remote execution Digest
func ProtoToDigest(pb proto.Message) (*remoteexecution.Digest, error) {
	wireFormat, err := proto.Marshal(pb)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(wireFormat)

	return &remoteexecution.Digest{
		Hash:      hex.EncodeToString(hash[:]),
		SizeBytes: int64(len(wireFormat)),
	}, nil
}
