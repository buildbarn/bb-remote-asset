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

// An actionCacheAssetStore uses an Action Cache to store the relation between
// asset references and assets.  The Remote Asset API associates an identifier
// (that is, URIs and Qualifiers) with an object in the CAS.  The Action Cache
// acts similarly, albeit with more metadata, associating an Action with an
// ActionResult.
//
// We can take advantage of this similarity by converting our URIs and Qualifiers
// to an Action, and our Asset to an ActionResult, and simply forwarding the
// request to an Action Cache.  Under this mode of operation, bb-remote-asset acts
// as a lightweight translation between the Remote Asset API and the Action Cache.
//
// The primary reason for this is to eliminate the requirement for bb-remote-asset
// to maintain state in its deployment, allowing for simpler operation.  It also
// means we can use the existing Action Cache implementation to e.g. ensure
// referential integrity between the references and the CAS.
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

// Convert an AssetReference to an Action, Command and Input Root Directory
//
// This does not interact with the CAS in any way.  If using this to upload
// an Action and ActionResult, then the Command and all objects returned must be
// uploaded to the CAS to ensure referential integrity.
func (rs *actionCacheAssetStore) assetReferenceToAction(ref *asset.AssetReference, digestFunction digest.Function) (*remoteexecution.Action, []proto.Message, error) {
	objects := []proto.Message{}

	// 1. Create a reference excluding the URIs.
	//    This is used to associate the Qualifiers with the Action, which
	//    must all match, but not the URIs, which can match individually.
	qr := NewAssetReference(nil, ref.Qualifiers)
	_, qrDigest, err := ProtoSerialise(qr, digestFunction)
	if err != nil {
		return nil, nil, err
	}
	objects = append(objects, qr)

	// 2. Construct a directory that contains the qualifiers as a file
	directory := &remoteexecution.Directory{
		Files: []*remoteexecution.FileNode{{
			Name:   "AssetReference",
			Digest: qrDigest.GetProto(),
		}},
	}
	_, directoryDigest, err := ProtoSerialise(directory, digestFunction)
	if err != nil {
		return nil, nil, err
	}
	objects = append(objects, directory)

	// 3. Create a Command and Action based on the URIs and Qualifiers
	var command *remoteexecution.Command
	var action *remoteexecution.Action
	if commandGenerator, err := qualifier.QualifiersToCommand(ref.Qualifiers); err != nil || len(ref.Uris) > 1 {
		// Can't generate a Command.  Use the URIs as arguments
		command = &remoteexecution.Command{
			Arguments:             ref.Uris,
			OutputPaths:           []string{"out"},
			OutputDirectoryFormat: remoteexecution.Command_TREE_AND_DIRECTORY,
		}
		_, commandDigest, err := ProtoSerialise(command, digestFunction)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, command)
		action = &remoteexecution.Action{
			CommandDigest:   commandDigest.GetProto(),
			InputRootDigest: directoryDigest.GetProto(),
		}
	} else {
		// Generate a command based on the qualifiers
		command := commandGenerator(ref.Uris[0])
		_, commandDigest, err := ProtoSerialise(command, digestFunction)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, command)
		action = &remoteexecution.Action{
			CommandDigest:   commandDigest.GetProto(),
			InputRootDigest: EmptyDigest(digestFunction).GetProto(),
		}
	}

	return action, objects, nil
}

// Convert an Asset to an ActionResult proto
//
// Any items that the ActionResult proto references are uploaded to the CAS
// as part of this method.  For example, directory assets must be converted to a
// Tree proto in order to be referenced by the ActionResult.  The Tree protos are
// uploaded in this method, so that the ActionResult returned has referential
// integrity with the CAS.
func (rs *actionCacheAssetStore) assetToActionResult(ctx context.Context, data *asset.Asset, digestFunction digest.Function) (*remoteexecution.ActionResult, error) {
	result := &remoteexecution.ActionResult{
		ExecutionMetadata: &remoteexecution.ExecutedActionMetadata{
			QueuedTimestamp: data.LastUpdated,
		},
	}

	if data.Type == asset.Asset_DIRECTORY {
		// If the asset is a Directory, then we need to convert it into
		// a Tree proto for the ActionResult.  Alas, the only way to
		// do this is to recursively follow the Directory protos from the
		// CAS.

		// 1. Get the asset from the CAS and parse it as a Directory
		digest, err := digestFunction.NewDigestFromProto(data.Digest)
		if err != nil {
			return nil, err
		}
		d, err := rs.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, rs.maximumMessageSizeBytes)
		if err != nil {
			// Users will hit this if they upload an digest referencing an
			// arbitary Blob in `PushDirectory` or a digest that does not
			// reference any blob at all.
			return nil, util.StatusWrapf(
				err,
				"digest in directory asset does not reference a Directory",
			)
		}
		directory := d.(*remoteexecution.Directory)

		// 2. Construct a Tree from the Directory and upload it to the CAS
		tree, err := rs.directoryToTree(ctx, directory, digestFunction)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Failed to convert directory to tree (one of the subdirs is not in the CAS?): %v", err)
		}

		treePb, treeDigest, err := ProtoSerialise(tree, digestFunction)
		if err != nil {
			return nil, err
		}
		err = rs.contentAddressableStorage.Put(ctx, treeDigest, treePb)
		if err != nil {
			return nil, err
		}

		// 3. Use the directory from the asset as the directory "out" in the ActionResult.
		result.OutputDirectories = []*remoteexecution.OutputDirectory{{
			Path:                "out",
			RootDirectoryDigest: data.Digest,
			TreeDigest:          treeDigest.GetProto(),
		}}
	} else if data.Type == asset.Asset_BLOB {
		// Simply use the digest as an Output File, called "out"
		result.OutputFiles = []*remoteexecution.OutputFile{{
			Path:   "out",
			Digest: data.Digest,
		}}
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "unknown asset Type %v", data.Type)
	}

	return result, nil
}

// Convert an ActionResult proto to an Asset
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

func (rs *actionCacheAssetStore) Get(ctx context.Context, ref *asset.AssetReference, digestFunction digest.Function) (*asset.Asset, error) {
	action, _, err := rs.assetReferenceToAction(ref, digestFunction)
	if err != nil {
		return nil, err
	}
	_, digest, err := ProtoSerialise(action, digestFunction)

	data, err := rs.actionCache.Get(ctx, digest).ToProto(
		&remoteexecution.ActionResult{},
		rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, util.StatusWrapf(err, "could not get action from action cache")
	}
	return rs.actionResultToAsset(data.(*remoteexecution.ActionResult))
}

func (rs *actionCacheAssetStore) Put(ctx context.Context, ref *asset.AssetReference, data *asset.Asset, digestFunction digest.Function) error {
	// Convert the AssetReference to an Action
	action, extraObjs, err := rs.assetReferenceToAction(ref, digestFunction)
	if err != nil {
		return err
	}

	// Upload the Action
	actionPb, actionDigest, err := ProtoSerialise(action, digestFunction)
	if err != nil {
		return err
	}
	err = rs.contentAddressableStorage.Put(ctx, actionDigest, actionPb)
	if err != nil {
		return err
	}

	// Upload extra things required for referential integrity
	for _, obj := range extraObjs {
		buf, digest, err := ProtoSerialise(obj, digestFunction)
		if err != nil {
			return err
		}
		err = rs.contentAddressableStorage.Put(ctx, digest, buf)
		if err != nil {
			return err
		}
	}

	// Convert the Asset to an ActionResult
	actionResult, err := rs.assetToActionResult(ctx, data, digestFunction)
	if err != nil {
		return err
	}

	// Upload to the ActionCache
	return rs.actionCache.Put(ctx, actionDigest, buffer.NewProtoBufferFromProto(actionResult, buffer.UserProvided))
}

// Utility method to convert a Directory Proto to a Tree Proto
func (rs *actionCacheAssetStore) directoryToTree(ctx context.Context, directory *remoteexecution.Directory, digestFunction digest.Function) (*remoteexecution.Tree, error) {
	children := []*remoteexecution.Directory{}
	for _, node := range directory.Directories {
		nodeChildren, err := rs.directoryNodeToDirectories(ctx, digestFunction, node)
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

// Utility method to list all descendants of a DirectoryNode, used for converting
// a Directory into a Tree
func (rs *actionCacheAssetStore) directoryNodeToDirectories(ctx context.Context, digestFunction digest.Function, node *remoteexecution.DirectoryNode) ([]*remoteexecution.Directory, error) {
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
		children, err := rs.directoryNodeToDirectories(ctx, digestFunction, node)
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
