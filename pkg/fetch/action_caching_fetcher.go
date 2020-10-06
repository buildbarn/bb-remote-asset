package fetch

import (
	"context"
	"log"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type actionCachingFetcher struct {
	fetcher           Fetcher
	pusher            remoteasset.PushServer
	actionCacheClient remoteexecution.ActionCacheClient
	requestTranslator translator.RequestTranslator
}

// NewActionCachingFetcher creates a new Fetcher suitable for farming Fetch requests to an Action Cache
func NewActionCachingFetcher(fetcher Fetcher, pusher remoteasset.PushServer, client grpc.ClientConnInterface) Fetcher {
	return &actionCachingFetcher{
		fetcher:           fetcher,
		pusher:            pusher,
		actionCacheClient: remoteexecution.NewActionCacheClient(client),
		requestTranslator: translator.RequestTranslator{},
	}
}

func (acf *actionCachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	action, _, err := acf.requestTranslator.FetchBlobToAction(req)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(&action)
	if err != nil {
		return nil, err
	}

	log.Printf("Looking in Action Cache: %v", req.Uris[0])
	actionResult, err := acf.actionCacheClient.GetActionResult(ctx, &remoteexecution.GetActionResultRequest{
		InstanceName: req.InstanceName,
		ActionDigest: actionDigest,
		InlineStdout: false,
		InlineStderr: false,
	})
	if err == nil {
		log.Printf("Found in Action Cache: %v", req.Uris[0])
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
	return nil, status.Error(codes.Unimplemented, "FetchDirectory not implemented yet!")
}

func (acf *actionCachingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Set{}
}
