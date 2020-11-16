package storage

import (
	"context"
	"log"
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

func (rs *actionCacheAssetStore) isDirectory(ctx context.Context, asset *asset.Asset, instance digest.InstanceName) error {
	digest, err := instance.NewDigestFromProto(asset.Digest)
	if err != nil {
		return err
	}
	_, err = rs.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, rs.maximumMessageSizeBytes)
	return err
}

func (rs *actionCacheAssetStore) actionResultToAsset(ctx context.Context, a *remoteexecution.ActionResult, instance digest.InstanceName) (*asset.Asset, error) {
	digest := &remoteexecution.Digest{}
	for _, dir := range a.OutputDirectories {
		if dir.Path == "out" {
			digest = dir.TreeDigest
		}
	}
	if (digest != &remoteexecution.Digest{}) {
		treeDigest, err := instance.NewDigestFromProto(digest)
		if err != nil {
			return nil, err
		}
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
	action, _, err := assetReferenceToAction(ref)
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
	action, command, err := assetReferenceToAction(ref)
	if err != nil {
		return err
	}
	log.Printf("Action: %v", action)
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
	err = rs.isDirectory(ctx, data, instance)
	if err == nil {
		tree, err := rs.directoryToTree(ctx, data.Digest, instance)
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
		if status.Code(err) != codes.InvalidArgument {
			return err
		}
		actionResult.OutputFiles = []*remoteexecution.OutputFile{{
			Path:   "out",
			Digest: data.Digest,
		}}
	}
	log.Printf("Action Result: %v", actionResult)
	return rs.actionCache.Put(ctx, bbActionDigest, buffer.NewProtoBufferFromProto(actionResult, buffer.UserProvided))
}

func (rs *actionCacheAssetStore) directoryToTree(ctx context.Context, d *remoteexecution.Digest, instance digest.InstanceName) (*remoteexecution.Tree, error) {
	digest, err := instance.NewDigestFromProto(d)
	if err != nil {
		return nil, err
	}
	directory, err := rs.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, rs.maximumMessageSizeBytes)
	if err != nil {
		return nil, err
	}
	children := []*remoteexecution.Directory{}
	for _, node := range directory.(*remoteexecution.Directory).Directories {
		nodeChildren, err := rs.directoryNodeToDirectories(ctx, instance, node)
		if err != nil {
			return nil, err
		}
		children = append(children, nodeChildren...)
	}

	return &remoteexecution.Tree{
		Root:     directory.(*remoteexecution.Directory),
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
