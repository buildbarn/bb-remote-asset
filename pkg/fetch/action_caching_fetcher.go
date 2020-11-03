package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
	"github.com/buildbarn/bb-remote-asset/pkg/translator"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type actionCachingFetcher struct {
	fetcher                   Fetcher
	pusher                    remoteasset.PushServer
	actionCache               blobstore.BlobAccess
	contentAddressableStorage blobstore.BlobAccess
	requestTranslator         translator.RequestTranslator
	maximumSizeBytes          int
}

// NewActionCachingFetcher creates a new Fetcher suitable for farming Fetch requests to an Action Cache
func NewActionCachingFetcher(fetcher Fetcher, pusher remoteasset.PushServer, actionCache, contentAddressableStorage blobstore.BlobAccess, maximumSizeBytes int) Fetcher {
	return &actionCachingFetcher{
		fetcher:                   fetcher,
		pusher:                    pusher,
		actionCache:               actionCache,
		contentAddressableStorage: contentAddressableStorage,
		requestTranslator:         translator.RequestTranslator{},
		maximumSizeBytes:          maximumSizeBytes,
	}
}

func (acf *actionCachingFetcher) checkActionCache(ctx context.Context, actionDigest digest.Digest, directory bool) (*remoteexecution.Digest, error) {
	actionResult, err := acf.actionCache.Get(ctx, actionDigest).ToProto(&remoteexecution.ActionResult{}, acf.maximumSizeBytes)
	if err != nil {
		return nil, err
	}
	blobDigest := translator.EmptyDigest
	if directory {
		for _, dir := range actionResult.(*remoteexecution.ActionResult).OutputDirectories {
			if dir.Path != "out" {
				continue
			}
			blobDigest = dir.TreeDigest
		}
		if blobDigest != translator.EmptyDigest {
			instanceName := actionDigest.GetInstanceName()
			digest, err := instanceName.NewDigestFromProto(blobDigest)
			if err != nil {
				return nil, err
			}
			tree, err := acf.contentAddressableStorage.Get(ctx, digest).ToProto(&remoteexecution.Tree{}, acf.maximumSizeBytes)
			if err != nil {
				return nil, err
			}
			root := tree.(*remoteexecution.Tree).Root
			blobDigest, err = translator.ProtoToDigest(root)
			if err != nil {
				return nil, err
			}
		}
	} else {
		for _, file := range actionResult.(*remoteexecution.ActionResult).OutputFiles {
			if file.Path != "out" {
				continue
			}
			blobDigest = file.Digest
		}
	}

	return blobDigest, nil
}

func (acf *actionCachingFetcher) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}

	action, _, err := acf.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(action)
	if err != nil {
		return nil, err
	}
	digest, err := instanceName.NewDigestFromProto(actionDigest)
	if err != nil {
		return nil, err
	}

	// Check for cached result with full list of URIs
	blobDigest, err := acf.checkActionCache(ctx, digest, false)
	if err == nil {
		return &remoteasset.FetchBlobResponse{
			Status:     status.New(codes.OK, "Blob fetched successfully from asset cache").Proto(),
			Uri:        req.Uris[0],
			Qualifiers: req.Qualifiers,
			BlobDigest: blobDigest,
		}, nil
	}

	// Check for cached result of each URI individually
	for _, uri := range req.Uris {
		action, _, err := acf.requestTranslator.URIsToAction([]string{uri})
		if err != nil {
			return nil, err
		}
		actionDigest, err := translator.ProtoToDigest(action)
		if err != nil {
			return nil, err
		}
		digest, err := instanceName.NewDigestFromProto(actionDigest)
		if err != nil {
			return nil, err
		}

		blobDigest, err := acf.checkActionCache(ctx, digest, false)
		if err == nil {
			return &remoteasset.FetchBlobResponse{
				Status:     status.New(codes.OK, "Blob fetched successfully from asset cache").Proto(),
				Uri:        uri,
				Qualifiers: req.Qualifiers,
				BlobDigest: blobDigest,
			}, nil
		}
	}

	// Blob wasn't found, fetch it
	response, err := acf.fetcher.FetchBlob(ctx, req)
	if err != nil {
		return response, err
	}

	// Push blob with successful URI
	pushReq := &remoteasset.PushBlobRequest{
		InstanceName: req.InstanceName,
		Uris:         []string{response.Uri},
		Qualifiers:   req.Qualifiers,
		BlobDigest:   response.BlobDigest,
	}
	_, err = acf.pusher.PushBlob(ctx, pushReq)
	if err != nil {
		return nil, err
	}

	if len(req.Uris) > 1 {
		// Push blob with all URIs from request
		pushReq.Uris = req.Uris
		_, err = acf.pusher.PushBlob(ctx, pushReq)
	}
	return response, err
}

func (acf *actionCachingFetcher) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, err
	}

	action, _, err := acf.requestTranslator.URIsToAction(req.Uris)
	if err != nil {
		return nil, err
	}
	actionDigest, err := translator.ProtoToDigest(action)
	if err != nil {
		return nil, err
	}
	digest, err := instanceName.NewDigestFromProto(actionDigest)
	if err != nil {
		return nil, err
	}

	// Check for cached result with full list of URIs
	dirDigest, err := acf.checkActionCache(ctx, digest, true)
	if err == nil {
		return &remoteasset.FetchDirectoryResponse{
			Status:              status.New(codes.OK, "Blob fetched successfully from asset cache").Proto(),
			Uri:                 req.Uris[0],
			Qualifiers:          req.Qualifiers,
			RootDirectoryDigest: dirDigest,
		}, nil
	}
	// Check for cached result with each URI individually
	for _, uri := range req.Uris {
		action, _, err := acf.requestTranslator.URIsToAction([]string{uri})
		if err != nil {
			return nil, err
		}
		actionDigest, err := translator.ProtoToDigest(action)
		if err != nil {
			return nil, err
		}

		digest, err := instanceName.NewDigestFromProto(actionDigest)
		if err != nil {
			return nil, err
		}
		dirDigest, err := acf.checkActionCache(ctx, digest, true)
		if err == nil {
			return &remoteasset.FetchDirectoryResponse{
				Status:              status.New(codes.OK, "Directory fetched successfully from asset cache").Proto(),
				Uri:                 req.Uris[0],
				Qualifiers:          req.Qualifiers,
				RootDirectoryDigest: dirDigest,
			}, nil
		}
	}

	// Directory wasn't found, fetch it
	response, err := acf.fetcher.FetchDirectory(ctx, req)
	if err != nil {
		return response, err
	}

	// Push directory with successful URI
	pushReq := &remoteasset.PushDirectoryRequest{
		InstanceName:        req.InstanceName,
		Uris:                []string{response.Uri},
		Qualifiers:          req.Qualifiers,
		RootDirectoryDigest: response.RootDirectoryDigest,
	}
	_, err = acf.pusher.PushDirectory(ctx, pushReq)
	if err != nil {
		return nil, err
	}

	if len(req.Uris) > 1 {
		// Push directory with all URIs from request
		pushReq.Uris = req.Uris
		_, err = acf.pusher.PushDirectory(ctx, pushReq)
	}
	return response, err
}

func (acf *actionCachingFetcher) CheckQualifiers(qualifiers qualifier.Set) qualifier.Set {
	return qualifier.Set{}
}
