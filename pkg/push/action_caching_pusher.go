package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

type actionCachingPusher struct {
	pusher                          remoteasset.PushServer
	actionCacheClient               remoteexecution.ActionCacheClient
	contentAddressableStorageClient remoteexecution.ContentAddressableStorageClient
	requestTranslator               translator.RequestTranslator
}

// NewActionCachingPusher creates a new Push server using the ActionCache as a backend
func NewActionCachingPusher(pusher remoteasset.PushServer, client grpc.ClientConnInterface) remoteasset.PushServer {
	return &actionCachingPusher{
		pusher:                          pusher,
		actionCacheClient:               remoteexecution.NewActionCacheClient(client),
		contentAddressableStorageClient: remoteexecution.NewContentAddressableStorageClient(client),
		requestTranslator:               translator.RequestTranslator{},
	}
}

func (acp *actionCachingPusher) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	action, command, err := acp.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionPb, err := proto.Marshal(&action)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(&action)
	if err != nil {
		return nil, err
	}

	commandPb, err := proto.Marshal(&command)
	if err != nil {
		return nil, err
	}
	commandDigest, err := translator.ProtoToDigest(&command)
	if err != nil {
		return nil, err
	}

	actionResult := acp.requestTranslator.PushBlobToActionResult(req)

	_, err = acp.contentAddressableStorageClient.BatchUpdateBlobs(ctx, &remoteexecution.BatchUpdateBlobsRequest{
		InstanceName: req.InstanceName,
		Requests: []*remoteexecution.BatchUpdateBlobsRequest_Request{
			&remoteexecution.BatchUpdateBlobsRequest_Request{
				Digest: actionDigest,
				Data:   actionPb,
			},
			&remoteexecution.BatchUpdateBlobsRequest_Request{
				Digest: commandDigest,
				Data:   commandPb,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	_, err = acp.actionCacheClient.UpdateActionResult(ctx, &remoteexecution.UpdateActionResultRequest{
		InstanceName: req.InstanceName,
		ActionDigest: actionDigest,
		ActionResult: &actionResult,
	})
	if err != nil {
		return nil, err
	}

	return acp.pusher.PushBlob(ctx, req)
}

func (acp *actionCachingPusher) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	action, command, err := acp.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionPb, err := proto.Marshal(&action)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(&action)
	if err != nil {
		return nil, err
	}

	commandPb, err := proto.Marshal(&command)
	if err != nil {
		return nil, err
	}
	commandDigest, err := translator.ProtoToDigest(&command)
	if err != nil {
		return nil, err
	}
	tree, err := acp.directoryToTree(ctx, req)
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

	_, err = acp.contentAddressableStorageClient.BatchUpdateBlobs(ctx, &remoteexecution.BatchUpdateBlobsRequest{
		InstanceName: req.InstanceName,
		Requests: []*remoteexecution.BatchUpdateBlobsRequest_Request{
			&remoteexecution.BatchUpdateBlobsRequest_Request{
				Digest: actionDigest,
				Data:   actionPb,
			},
			&remoteexecution.BatchUpdateBlobsRequest_Request{
				Digest: commandDigest,
				Data:   commandPb,
			},
			&remoteexecution.BatchUpdateBlobsRequest_Request{
				Digest: treeDigest,
				Data:   treePb,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	_, err = acp.actionCacheClient.UpdateActionResult(ctx, &remoteexecution.UpdateActionResultRequest{
		InstanceName: req.InstanceName,
		ActionDigest: actionDigest,
		ActionResult: &actionResult,
	})
	if err != nil {
		return nil, err
	}

	return acp.pusher.PushDirectory(ctx, req)
}

func (acp *actionCachingPusher) directoryToTree(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteexecution.Tree, error) {
	readResponse, err := acp.contentAddressableStorageClient.BatchReadBlobs(ctx, &remoteexecution.BatchReadBlobsRequest{
		InstanceName: req.InstanceName,
		Digests:      []*remoteexecution.Digest{req.RootDirectoryDigest}})
	if err != nil {
		return nil, err
	}
	directory := &remoteexecution.Directory{}
	err = proto.Unmarshal(readResponse.Responses[0].Data, directory)
	if err != nil {
		return nil, err
	}
	children := []*remoteexecution.Directory{}
	for _, node := range directory.Directories {
		nodeChildren, err := acp.directoryNodeToDirectories(ctx, req.InstanceName, node)
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

func (acp *actionCachingPusher) directoryNodeToDirectories(ctx context.Context, instance string, node *remoteexecution.DirectoryNode) ([]*remoteexecution.Directory, error) {
	readResponse, err := acp.contentAddressableStorageClient.BatchReadBlobs(ctx, &remoteexecution.BatchReadBlobsRequest{
		InstanceName: instance,
		Digests:      []*remoteexecution.Digest{node.Digest}})
	if err != nil {
		return nil, err
	}
	directory := &remoteexecution.Directory{}
	err = proto.Unmarshal(readResponse.Responses[0].Data, directory)
	if err != nil {
		return nil, err
	}
	directories := []*remoteexecution.Directory{directory}
	for _, node := range directory.Directories {
		children, err := acp.directoryNodeToDirectories(ctx, instance, node)
		if err != nil {
			return nil, err
		}
		directories = append(directories, children...)
	}
	return directories, nil
}
