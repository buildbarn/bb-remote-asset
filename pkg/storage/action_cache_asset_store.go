package storage

import (
	"context"
	"time"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type actionCacheAssetStore struct {
	actionCache               blobstore.BlobAccess
	contentAddressableStorage blobstore.BlobAccess
	maximumMessageSizeBytes   int
}

// NewActionCacheAssetStore creates a new AssetStore which stores it's
// references as ActionResults in the Action Cache.
func NewActionCacheAssetStore(actionCache, contentAddressableStorage blobstore.BlobAccess, maximumMessageSizeBytes int) AssetStore {
	return &actionCacheAssetStore{
		actionCache:               actionCache,
		contentAddressableStorage: contentAddressableStorage,
		maximumMessageSizeBytes:   maximumMessageSizeBytes,
	}
}

func (rs *actionCacheAssetStore) assetToDirectory(ctx context.Context, asset *asset.Asset, instance digest.InstanceName) (*remoteexecution.Directory, error) {
	digest, err := instance.NewDigestFromProto(asset.Digest)
	if err != nil {
		return nil, err
	}
	directory, err := rs.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	return directory.(*remoteexecution.Directory), nil
}

func (rs *actionCacheAssetStore) actionResultToAsset(ctx context.Context, a *remoteexecution.ActionResult, instance digest.InstanceName) (*asset.Asset, error) {
	digest := &remoteexecution.Digest{}
	// Check if there is an output directory in the action result
	for _, dir := range a.OutputDirectories {
		if dir.Path == "out" {
			digest = dir.TreeDigest
		}
	}
	// If the required output directory is present
	if digest.Hash != "" {
		treeDigest, err := instance.NewDigestFromProto(digest)
		if err != nil {
			return nil, err
		}
		// The action result contains a tree digest, but a directory digest
		// is needed, so retrieve the tree message and get the digest of the
		// root
		tree, err := rs.contentAddressableStorage.Get(ctx, treeDigest).ToProto(&remoteexecution.Tree{}, rs.maximumMessageSizeBytes)
		if err != nil {
			return nil, err
		}
		root := tree.(*remoteexecution.Tree).Root
		digest, err = ProtoToDigest(root)
		if err != nil {
			return nil, err
		}
	} else {
		// Required output directory is not present, look for required
		// output file
		for _, file := range a.OutputFiles {
			if file.Path == "out" {
				digest = file.Digest
			}
		}
	}
	return &asset.Asset{
		Digest:      digest,
		ExpireAt:    getDefaultTimestamp(),
		LastUpdated: a.ExecutionMetadata.QueuedTimestamp,
	}, nil
}

func (rs *actionCacheAssetStore) Get(ctx context.Context, ref *asset.AssetReference, instance digest.InstanceName) (*asset.Asset, error) {
	// Create asset reference using only the qualifiers of the request
	qualifierReference := NewAssetReference(nil, ref.Qualifiers)
	refDigest, err := ProtoToDigest(qualifierReference)
	if err != nil {
		return nil, err
	}
	// Construct a directory using the reference of only qualifiers
	directory := &remoteexecution.Directory{
		Files: []*remoteexecution.FileNode{{
			Name:   "AssetReference",
			Digest: refDigest,
		}},
	}
	directoryDigest, err := ProtoToDigest(directory)
	if err != nil {
		return nil, err
	}
	// Create an action using the asset ref and directory containing
	// qualifiers
	action, _, err := assetReferenceToAction(ref, directoryDigest)
	if err != nil {
		return nil, err
	}
	actionDigest, err := ProtoToDigest(action)
	if err != nil {
		return nil, err
	}
	digest, err := instance.NewDigestFromProto(actionDigest)
	if err != nil {
		return nil, err
	}

	data, err := rs.actionCache.Get(ctx, digest).ToProto(
		&remoteexecution.ActionResult{},
		rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	return rs.actionResultToAsset(ctx, data.(*remoteexecution.ActionResult), instance)
}

func (rs *actionCacheAssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, instance digest.InstanceName) error {
	// Create asset reference using only the qualifiers of the request
	qualifierReference := NewAssetReference(nil, ref.Qualifiers)
	refDigest, err := ProtoToDigest(qualifierReference)
	if err != nil {
		return err
	}
	refPb, err := proto.Marshal(qualifierReference)
	if err != nil {
		return err
	}
	bbRefDigest, err := AssetReferenceToDigest(qualifierReference, instance)
	if err != nil {
		return err
	}
	// Put the qualifier reference in the CAS to ensure completeness of
	// the action result
	err = rs.contentAddressableStorage.Put(ctx, bbRefDigest, buffer.NewCASBufferFromByteSlice(bbRefDigest, refPb, buffer.UserProvided))
	if err != nil {
		return err
	}
	// Construct a directory using the reference of only qualifiers
	// This is how qualifiers are linked to the assets when stored as
	// action results
	directory := &remoteexecution.Directory{
		Files: []*remoteexecution.FileNode{{
			Name:   "AssetReference",
			Digest: refDigest,
		}},
	}
	directoryPb, err := proto.Marshal(directory)
	if err != nil {
		return err
	}
	directoryDigest, err := ProtoToDigest(directory)
	if err != nil {
		return err
	}
	bbDirectoryDigest, err := instance.NewDigestFromProto(directoryDigest)
	if err != nil {
		return nil
	}
	err = rs.contentAddressableStorage.Put(ctx, bbDirectoryDigest, buffer.NewCASBufferFromByteSlice(bbDirectoryDigest, directoryPb, buffer.UserProvided))
	if err != nil {
		return err
	}
	// Create the action with the qualifier directory as the input root
	action, command, err := assetReferenceToAction(ref, directoryDigest)
	if err != nil {
		return err
	}
	actionPb, err := proto.Marshal(action)
	if err != nil {
		return err
	}
	actionDigest, err := ProtoToDigest(action)
	if err != nil {
		return err
	}
	bbActionDigest, err := instance.NewDigestFromProto(actionDigest)
	if err != nil {
		return err
	}
	err = rs.contentAddressableStorage.Put(ctx, bbActionDigest, buffer.NewCASBufferFromByteSlice(bbActionDigest, actionPb, buffer.UserProvided))
	if err != nil {
		return err
	}

	commandPb, err := proto.Marshal(command)
	if err != nil {
		return err
	}
	commandDigest, err := ProtoToDigest(command)
	if err != nil {
		return err
	}
	bbCommandDigest, err := instance.NewDigestFromProto(commandDigest)
	if err != nil {
		return err
	}
	err = rs.contentAddressableStorage.Put(ctx, bbCommandDigest, buffer.NewCASBufferFromByteSlice(bbCommandDigest, commandPb, buffer.UserProvided))
	if err != nil {
		return err
	}

	actionResult := &remoteexecution.ActionResult{
		ExecutionMetadata: &remoteexecution.ExecutedActionMetadata{
			QueuedTimestamp: data.LastUpdated,
		},
	}

	// Check if the input asset is a directory or blob
	d, err := rs.assetToDirectory(ctx, data, instance)
	if err == nil {
		// If it is a directory, construct a tree from it as tree digest is
		// required for action result
		tree, err := rs.directoryToTree(ctx, d, instance)
		if err != nil {
			return err
		}
		treePb, err := proto.Marshal(tree)
		if err != nil {
			return err
		}
		treeDigest, err := ProtoToDigest(tree)
		if err != nil {
			return err
		}
		bbTreeDigest, err := instance.NewDigestFromProto(treeDigest)
		if err != nil {
			return err
		}
		err = rs.contentAddressableStorage.Put(ctx, bbTreeDigest, buffer.NewCASBufferFromByteSlice(bbTreeDigest, treePb, buffer.UserProvided))
		if err != nil {
			return err
		}
		actionResult.OutputDirectories = []*remoteexecution.OutputDirectory{{
			Path:       "out",
			TreeDigest: treeDigest,
		}}
	} else {
		// If it isn't a directory, use the digest as an output file digest
		if status.Code(err) != codes.InvalidArgument {
			return err
		}
		actionResult.OutputFiles = []*remoteexecution.OutputFile{{
			Path:   "out",
			Digest: data.Digest,
		}}
	}
	return rs.actionCache.Put(ctx, bbActionDigest, buffer.NewProtoBufferFromProto(actionResult, buffer.UserProvided))
}

func (rs *actionCacheAssetStore) directoryToTree(ctx context.Context, directory *remoteexecution.Directory, instance digest.InstanceName) (*remoteexecution.Tree, error) {
	children := []*remoteexecution.Directory{}
	for _, node := range directory.Directories {
		nodeChildren, err := rs.directoryNodeToDirectories(ctx, instance, node)
		if err != nil {
			return nil, err
		}
		children = append(children, nodeChildren...)
	}

	return &remoteexecution.Tree{
		Root:     directory,
		Children: children,
	}, nil
}

func (rs *actionCacheAssetStore) directoryNodeToDirectories(ctx context.Context, instance digest.InstanceName, node *remoteexecution.DirectoryNode) ([]*remoteexecution.Directory, error) {
	digest, err := instance.NewDigestFromProto(node.Digest)
	if err != nil {
		return nil, err
	}
	directory, err := rs.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	directories := []*remoteexecution.Directory{directory.(*remoteexecution.Directory)}
	for _, node := range directory.(*remoteexecution.Directory).Directories {
		children, err := rs.directoryNodeToDirectories(ctx, instance, node)
		if err != nil {
			return nil, err
		}
		directories = append(directories, children...)
	}
	return directories, nil
}

func getDefaultTimestamp() *timestamp.Timestamp {
	ts, _ := ptypes.TimestampProto(time.Unix(0, 0))
	return ts
}
