package storage

import (
	"context"
	"time"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(asset.Digest.GetHash()))
	if err != nil {
		return nil, err
	}
	digest, err := digestFunction.NewDigestFromProto(asset.Digest)
	if err != nil {
		return nil, err
	}
	directory, err := rs.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	return directory.(*remoteexecution.Directory), nil
}

func (rs *actionCacheAssetStore) actionResultToAsset(a *remoteexecution.ActionResult) (*asset.Asset, error) {
	digest := &remoteexecution.Digest{}
	assetType := asset.Asset_DIRECTORY

	// Check if there is an output directory in the action result
	for _, dir := range a.OutputDirectories {
		if dir.Path == "out" {
			digest = dir.RootDirectoryDigest
		}
	}

	if digest == nil || digest.Hash == "" {
		assetType = asset.Asset_BLOB
		// Required output directory is not present, look for required
		// output file
		for _, file := range a.OutputFiles {
			if file.Path == "out" {
				digest = file.Digest
			}
		}
	}

	if digest == nil || digest.Hash == "" {
		return nil, status.Errorf(codes.InvalidArgument, "could not find digest (either directory or blob) in ActionResult")
	}

	return &asset.Asset{
		Digest:      digest,
		ExpireAt:    getDefaultTimestamp(),
		LastUpdated: a.ExecutionMetadata.QueuedTimestamp,
		Type:        assetType,
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
	var action *remoteexecution.Action
	if commandGenerator, err := qualifier.QualifiersToCommand(ref.Qualifiers); err != nil || len(ref.Uris) > 1 {
		// Create the action with the qualifier directory as the input root
		action, _, err = assetReferenceToAction(ref, directoryDigest)
		if err != nil {
			return nil, err
		}
	} else {
		command := commandGenerator(ref.Uris[0])
		commandDigest, err := ProtoToDigest(command)
		if err != nil {
			return nil, err
		}
		action = &remoteexecution.Action{
			CommandDigest:   commandDigest,
			InputRootDigest: EmptyDigest,
		}
	}
	actionDigest, err := ProtoToDigest(action)
	if err != nil {
		return nil, err
	}
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(actionDigest.GetHash()))
	if err != nil {
		return nil, err
	}
	digest, err := digestFunction.NewDigestFromProto(actionDigest)
	if err != nil {
		return nil, err
	}

	data, err := rs.actionCache.Get(ctx, digest).ToProto(
		&remoteexecution.ActionResult{},
		rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, util.StatusWrapf(err, "could not get action from action cache")
	}
	return rs.actionResultToAsset(data.(*remoteexecution.ActionResult))
}

func (rs *actionCacheAssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, instance digest.InstanceName) error {
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(data.GetDigest().GetHash()))
	if err != nil {
		return err
	}
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
	bbDirectoryDigest, err := digestFunction.NewDigestFromProto(directoryDigest)
	if err != nil {
		return nil
	}
	err = rs.contentAddressableStorage.Put(ctx, bbDirectoryDigest, buffer.NewCASBufferFromByteSlice(bbDirectoryDigest, directoryPb, buffer.UserProvided))
	if err != nil {
		return err
	}
	var action *remoteexecution.Action
	var command *remoteexecution.Command
	if commandGenerator, err := qualifier.QualifiersToCommand(ref.Qualifiers); err != nil || len(ref.Uris) > 1 {
		// Create the action with the qualifier directory as the input root
		action, command, err = assetReferenceToAction(ref, directoryDigest)
		if err != nil {
			return err
		}
	} else {
		command = commandGenerator(ref.Uris[0])
		commandDigest, err := ProtoToDigest(command)
		if err != nil {
			return err
		}
		action = &remoteexecution.Action{
			CommandDigest:   commandDigest,
			InputRootDigest: EmptyDigest,
		}
	}
	actionPb, err := proto.Marshal(action)
	if err != nil {
		return err
	}
	actionDigest, err := ProtoToDigest(action)
	if err != nil {
		return err
	}
	bbActionDigest, err := digestFunction.NewDigestFromProto(actionDigest)
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
	bbCommandDigest, err := digestFunction.NewDigestFromProto(commandDigest)
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

	if data.Type == asset.Asset_DIRECTORY {
		d, err := rs.assetToDirectory(ctx, data, instance)
		if err != nil {
			// Users will hit this if they upload an digest referencing an
			// arbitary Blob in `PushDirectory` or a digest that does not
			// reference any blob at all.
			return util.StatusWrapf(
				err,
				"digest in directory asset does not reference a Directory",
			)
		}

		// If it is a directory, construct a tree from it as tree digest is
		// required for action result
		tree, err := rs.directoryToTree(ctx, d, instance)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "Failed to convert directory to tree (one of the subdirs is not in the CAS?): %v", err)
		}
		treePb, err := proto.Marshal(tree)
		if err != nil {
			return err
		}
		treeDigest, err := ProtoToDigest(tree)
		if err != nil {
			return err
		}
		bbTreeDigest, err := digestFunction.NewDigestFromProto(treeDigest)
		if err != nil {
			return err
		}
		err = rs.contentAddressableStorage.Put(ctx, bbTreeDigest, buffer.NewCASBufferFromByteSlice(bbTreeDigest, treePb, buffer.UserProvided))
		if err != nil {
			return err
		}

		// Use digest as a root directory digest
		actionResult.OutputDirectories = []*remoteexecution.OutputDirectory{{
			Path:                "out",
			RootDirectoryDigest: data.Digest,
			TreeDigest:          treeDigest,
		}}
	} else if data.Type == asset.Asset_BLOB {
		// Use the digest as an output file digest
		actionResult.OutputFiles = []*remoteexecution.OutputFile{{
			Path:   "out",
			Digest: data.Digest,
		}}
	} else {
		return status.Errorf(codes.InvalidArgument, "unknown asset type %v", data.Type)
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
	digestFunction, err := instance.GetDigestFunction(remoteexecution.DigestFunction_UNKNOWN, len(node.GetDigest().GetHash()))
	if err != nil {
		return nil, err
	}
	digest, err := digestFunction.NewDigestFromProto(node.Digest)
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

func getDefaultTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(0, 0))
}
