package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type actionCachingFetcher struct {
	instanceName      string
	actionCacheClient remoteexecution.ActionCacheClient
	requestTranslator translator.RequestTranslator
}

// NewActionCachingFetcher creates a new Fetcher suitable for farming Fetch requests to an Action Cache
func NewActionCachingFetcher(instanceName string, client grpc.ClientConnInterface) Fetcher {
	return &actionCachingFetcher{
		instanceName:      instanceName,
		actionCacheClient: remoteexecution.NewActionCacheClient(client),
		requestTranslator: translator.RequestTranslator{},
	}
}

func (ac *actionCachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	action, _, err := ac.requestTranslator.FetchBlobToAction(req)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(&action)
	if err != nil {
		return nil, err
	}

	actionResult, err := ac.actionCacheClient.GetActionResult(ctx, &remoteexecution.GetActionResultRequest{
		InstanceName: ac.instanceName,
		ActionDigest: actionDigest,
		InlineStdout: false,
		InlineStderr: false,
	})
	if err != nil {
		return nil, err
	}

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

func (ac *actionCachingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return nil, status.Error(codes.Unimplemented, "FetchDirectory not implemented yet!")
}

func (ac *actionCachingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Set{}
}
