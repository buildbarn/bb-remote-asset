package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetPushServer struct {
	assetStore               storage.AssetStore
	allowUpdatesForInstances map[digest.InstanceName]bool
}

// getDigestFunction is a convenient wrapper to get a Buildbarn digest Function
// based on the instance name.  If digestFunction is unknown, then we attempt to
// guess the function based on the length of sentDigest's hash.
func getDigestFunction(digestFunction remoteexecution.DigestFunction_Value, instanceName string, sentDigest *remoteexecution.Digest) (digest.Function, error) {
	instance, err := digest.NewInstanceName(instanceName)
	if err != nil {
		return digest.Function{}, util.StatusWrapf(err, "Invalid instance name %#v", instanceName)
	}

	return instance.GetDigestFunction(digestFunction, len(sentDigest.GetHash()))
}

// NewAssetPushServer creates a gRPC service for serving the contents
// of a Remote Asset Push server.
func NewAssetPushServer(AssetStore storage.AssetStore, allowUpdatesForInstances map[digest.InstanceName]bool) remoteasset.PushServer {
	return &assetPushServer{
		assetStore:               AssetStore,
		allowUpdatesForInstances: allowUpdatesForInstances,
	}
}

func (s *assetPushServer) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "PushBlob requires at least one URI")
	}

	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName, req.BlobDigest)
	if err != nil {
		return nil, err
	}

	if !s.allowUpdatesForInstances[digestFunction.GetInstanceName()] {
		return nil, status.Errorf(codes.PermissionDenied, "This service does not accept Blobs for instance %#v", req.InstanceName)
	}

	assetRef := storage.NewAssetReference(req.Uris, req.Qualifiers)
	assetData := storage.NewBlobAsset(req.BlobDigest, req.ExpireAt)
	err = s.assetStore.Put(ctx, assetRef, assetData, digestFunction)
	if err != nil {
		return nil, err
	}

	if len(req.Uris) > 1 {
		for _, uri := range req.Uris {
			assetRef := storage.NewAssetReference([]string{uri}, req.Qualifiers)
			assetData := storage.NewBlobAsset(req.BlobDigest, req.ExpireAt)
			err = s.assetStore.Put(ctx, assetRef, assetData, digestFunction)
			if err != nil {
				return nil, err
			}
		}
	}
	return &remoteasset.PushBlobResponse{}, nil
}

func (s *assetPushServer) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	if len(req.Uris) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "PushDirectory requires at least one URI")
	}

	digestFunction, err := getDigestFunction(req.DigestFunction, req.InstanceName, req.RootDirectoryDigest)
	if err != nil {
		return nil, err
	}

	if !s.allowUpdatesForInstances[digestFunction.GetInstanceName()] {
		return nil, status.Errorf(codes.PermissionDenied, "This service does not accept Directories for instance %#v", req.InstanceName)
	}

	assetRef := storage.NewAssetReference(req.Uris, req.Qualifiers)
	assetData := storage.NewDirectoryAsset(req.RootDirectoryDigest, req.ExpireAt)
	err = s.assetStore.Put(ctx, assetRef, assetData, digestFunction)
	if err != nil {
		return nil, err
	}

	if len(req.Uris) > 1 {
		for _, uri := range req.Uris {
			assetRef := storage.NewAssetReference([]string{uri}, req.Qualifiers)
			assetData := storage.NewDirectoryAsset(req.RootDirectoryDigest, req.ExpireAt)
			err = s.assetStore.Put(ctx, assetRef, assetData, digestFunction)
			if err != nil {
				return nil, err
			}
		}
	}
	return &remoteasset.PushDirectoryResponse{}, nil
}
