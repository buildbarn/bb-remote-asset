package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
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

	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	if !s.allowUpdatesForInstances[instanceName] {
		return nil, status.Errorf(codes.PermissionDenied, "This service does not accept Blobs for instance %#v", req.InstanceName)
	}

	assetRef := storage.NewAssetReference(req.Uris, req.Qualifiers)
	assetData := storage.NewBlobAsset(req.BlobDigest, req.ExpireAt)
	err = s.assetStore.Put(ctx, assetRef, assetData, instanceName)
	if err != nil {
		return nil, err
	}

	if len(req.Uris) > 1 {
		for _, uri := range req.Uris {
			assetRef := storage.NewAssetReference([]string{uri}, req.Qualifiers)
			assetData := storage.NewBlobAsset(req.BlobDigest, req.ExpireAt)
			err = s.assetStore.Put(ctx, assetRef, assetData, instanceName)
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

	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	if !s.allowUpdatesForInstances[instanceName] {
		return nil, status.Errorf(codes.PermissionDenied, "This service does not accept Directories for instance %#v", req.InstanceName)
	}

	assetRef := storage.NewAssetReference(req.Uris, req.Qualifiers)
	assetData := storage.NewDirectoryAsset(req.RootDirectoryDigest, req.ExpireAt)
	err = s.assetStore.Put(ctx, assetRef, assetData, instanceName)
	if err != nil {
		return nil, err
	}

	if len(req.Uris) > 1 {
		for _, uri := range req.Uris {
			assetRef := storage.NewAssetReference([]string{uri}, req.Qualifiers)
			assetData := storage.NewDirectoryAsset(req.RootDirectoryDigest, req.ExpireAt)
			err = s.assetStore.Put(ctx, assetRef, assetData, instanceName)
			if err != nil {
				return nil, err
			}
		}
	}
	return &remoteasset.PushDirectoryResponse{}, nil
}
