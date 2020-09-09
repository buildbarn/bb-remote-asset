package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	action, command, err := acp.requestTranslator.PushBlobToAction(req)
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
	return nil, status.Error(codes.Unimplemented, "PushDirectory not implemented yet!")
}
