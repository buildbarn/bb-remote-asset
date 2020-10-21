package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type actionCachingFetcher struct {
	fetcher                         Fetcher
	pusher                          remoteasset.PushServer
	actionCacheClient               remoteexecution.ActionCacheClient
	contentAddressableStorageClient remoteexecution.ContentAddressableStorageClient
	requestTranslator               translator.RequestTranslator
}

// NewActionCachingFetcher creates a new Fetcher suitable for farming Fetch requests to an Action Cache
func NewActionCachingFetcher(fetcher Fetcher, pusher remoteasset.PushServer, client grpc.ClientConnInterface) Fetcher {
	return &actionCachingFetcher{
		fetcher:                         fetcher,
		pusher:                          pusher,
		actionCacheClient:               remoteexecution.NewActionCacheClient(client),
		contentAddressableStorageClient: remoteexecution.NewContentAddressableStorageClient(client),
		requestTranslator:               translator.RequestTranslator{},
	}
}

func (acf *actionCachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	action, _, err := acf.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(&action)
	if err != nil {
		return nil, err
	}

	actionResult, err := acf.actionCacheClient.GetActionResult(ctx, &remoteexecution.GetActionResultRequest{
		InstanceName: req.InstanceName,
		ActionDigest: actionDigest,
		InlineStdout: false,
		InlineStderr: false,
	})
	if err == nil {
		blobDigest := translator.EmptyDigest
		for _, file := range actionResult.OutputFiles {
			if file.Path != "out" {
				continue
			}
			blobDigest = file.Digest
		}

		return &remoteasset.FetchBlobResponse{
			Status:     status.New(codes.OK, "Blob fetched successfully from asset cache").Proto(),
			Uri:        req.Uris[0],
			Qualifiers: req.Qualifiers,
			BlobDigest: blobDigest,
		}, nil
	}
	response, err := acf.fetcher.FetchBlob(ctx, req)
	if err != nil {
		return response, err
	}
	pushReq := &remoteasset.PushBlobRequest{
		InstanceName: req.InstanceName,
		Uris:         req.Uris,
		Qualifiers:   req.Qualifiers,
		BlobDigest:   response.BlobDigest,
	}
	_, err = acf.pusher.PushBlob(ctx, pushReq)
	return response, err
}

func (acf *actionCachingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	action, _, err := acf.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(&action)
	if err != nil {
		return nil, err
	}

	actionResult, err := acf.actionCacheClient.GetActionResult(ctx, &remoteexecution.GetActionResultRequest{
		InstanceName: req.InstanceName,
		ActionDigest: actionDigest,
		InlineStdout: false,
		InlineStderr: false,
	})
	if err == nil {
		dirDigest := translator.EmptyDigest
		treeDigest := translator.EmptyDigest
		for _, dir := range actionResult.OutputDirectories {
			if dir.Path != "out" {
				continue
			}
			treeDigest = dir.TreeDigest
		}
		if treeDigest != translator.EmptyDigest {
			readResponse, err := acf.contentAddressableStorageClient.BatchReadBlobs(ctx, &remoteexecution.BatchReadBlobsRequest{
				InstanceName: req.InstanceName,
				Digests:      []*remoteexecution.Digest{treeDigest},
			})
			if err != nil {
				return nil, err
			}
			tree := &remoteexecution.Tree{}
			err = proto.Unmarshal(readResponse.Responses[0].Data, tree)
			if err != nil {
				return nil, err
			}
			dirDigest, err = translator.ProtoToDigest(tree.Root)
			if err != nil {
				return nil, err
			}
		}

		return &remoteasset.FetchDirectoryResponse{
			Status:              status.New(codes.OK, "Directory fetched successfully from asset cache").Proto(),
			Uri:                 req.Uris[0],
			Qualifiers:          req.Qualifiers,
			RootDirectoryDigest: dirDigest,
		}, nil
	}
	return nil, status.Error(codes.NotFound, "Directory not found in Asset Cache")
}

func (acf *actionCachingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Set{}
}
