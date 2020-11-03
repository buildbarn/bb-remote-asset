package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/protobuf/proto"
)

type actionCachingPusher struct {
	pusher                    remoteasset.PushServer
	actionCache               blobstore.BlobAccess
	contentAddressableStorage blobstore.BlobAccess
	requestTranslator         translator.RequestTranslator
	maximumSizeBytes          int
}

// NewActionCachingPusher creates a new Push server using the ActionCache as a backend
func NewActionCachingPusher(pusher remoteasset.PushServer, actionCache, contentAddressableStorage blobstore.BlobAccess, maximumSizeBytes int) remoteasset.PushServer {
	return &actionCachingPusher{
		pusher:                    pusher,
		actionCache:               actionCache,
		contentAddressableStorage: contentAddressableStorage,
		requestTranslator:         translator.RequestTranslator{},
		maximumSizeBytes:          maximumSizeBytes,
	}
}

func (acp *actionCachingPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	action, command, err := acp.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionPb, err := proto.Marshal(action)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(action)
	if err != nil {
		return nil, err
	}

	commandPb, err := proto.Marshal(command)
	if err != nil {
		return nil, err
	}
	commandDigest, err := translator.ProtoToDigest(command)
	if err != nil {
		return nil, err
	}

	actionResult := acp.requestTranslator.PushBlobToActionResult(req)

	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	bbActionDigest, err := instanceName.NewDigestFromProto(actionDigest)
	if err != nil {
		return nil, err
	}
	bbCommandDigest, err := instanceName.NewDigestFromProto(commandDigest)
	if err != nil {
		return nil, err
	}
	err = acp.contentAddressableStorage.Put(ctx, bbActionDigest, buffer.NewCASBufferFromByteSlice(bbActionDigest, actionPb, buffer.UserProvided))
	if err != nil {
		return nil, err
	}
	err = acp.contentAddressableStorage.Put(ctx, bbCommandDigest, buffer.NewCASBufferFromByteSlice(bbCommandDigest, commandPb, buffer.UserProvided))
	if err != nil {
		return nil, err
	}

	err = acp.actionCache.Put(ctx, bbActionDigest, buffer.NewProtoBufferFromProto(&actionResult, buffer.UserProvided))
	if err != nil {
		return nil, err
	}

	return acp.pusher.PushBlob(ctx, req)
}

func (acp *actionCachingPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	action, command, err := acp.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionPb, err := proto.Marshal(action)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(action)
	if err != nil {
		return nil, err
	}
	commandPb, err := proto.Marshal(command)
	if err != nil {
		return nil, err
	}
	commandDigest, err := translator.ProtoToDigest(command)
	if err != nil {
		return nil, err
	}
	tree, err := acp.directoryToTree(ctx, req)
	if err != nil {
		return nil, err
	}
	treePb, err := proto.Marshal(tree)
	if err != nil {
		return nil, err
	}
	treeDigest, err := translator.ProtoToDigest(tree)
	if err != nil {
		return nil, err
	}

	actionResult := acp.requestTranslator.PushDirectoryToActionResult(req, treeDigest)
	if err != nil {
		return nil, err
	}
	bbActionDigest, err := instanceName.NewDigestFromProto(actionDigest)
	if err != nil {
		return nil, err
	}
	bbCommandDigest, err := instanceName.NewDigestFromProto(commandDigest)
	if err != nil {
		return nil, err
	}
	bbTreeDigest, err := instanceName.NewDigestFromProto(treeDigest)
	if err != nil {
		return nil, err
	}

	err = acp.contentAddressableStorage.Put(ctx, bbActionDigest, buffer.NewCASBufferFromByteSlice(bbActionDigest, actionPb, buffer.UserProvided))
	if err != nil {
		return nil, err
	}
	err = acp.contentAddressableStorage.Put(ctx, bbCommandDigest, buffer.NewCASBufferFromByteSlice(bbCommandDigest, commandPb, buffer.UserProvided))
	if err != nil {
		return nil, err
	}
	err = acp.contentAddressableStorage.Put(ctx, bbTreeDigest, buffer.NewCASBufferFromByteSlice(bbTreeDigest, treePb, buffer.UserProvided))
	if err != nil {
		return nil, err
	}

	err = acp.actionCache.Put(ctx, bbActionDigest, buffer.NewProtoBufferFromProto(&actionResult, buffer.UserProvided))
	if err != nil {
		return nil, err
	}

	return acp.pusher.PushDirectory(ctx, req)
}

func (acp *actionCachingPusher) directoryToTree(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteexecution.Tree, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}
	digest, err := instanceName.NewDigestFromProto(req.RootDirectoryDigest)
	if err != nil {
		return nil, err
	}
	directory, err := acp.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, acp.maximumSizeBytes)
	if err != nil {
		return nil, err
	}
	children := []*remoteexecution.Directory{}
	for _, node := range directory.(*remoteexecution.Directory).Directories {
		nodeChildren, err := acp.directoryNodeToDirectories(ctx, instanceName, node)
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

func (acp *actionCachingPusher) directoryNodeToDirectories(ctx context.Context, instanceName digest.InstanceName, node *remoteexecution.DirectoryNode) ([]*remoteexecution.Directory, error) {
	digest, err := instanceName.NewDigestFromProto(node.Digest)
	if err != nil {
		return nil, err
	}
	directory, err := acp.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Directory{}, acp.maximumSizeBytes)
	if err != nil {
		return nil, err
	}
	directories := []*remoteexecution.Directory{directory.(*remoteexecution.Directory)}
	for _, node := range directory.(*remoteexecution.Directory).Directories {
		children, err := acp.directoryNodeToDirectories(ctx, instanceName, node)
		if err != nil {
			return nil, err
		}
		directories = append(directories, children...)
	}
	return directories, nil
}
