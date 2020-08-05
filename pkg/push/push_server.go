package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetPushServer struct {
	referenceStore           *storage.ReferenceStore
	allowUpdatesForInstances map[digest.InstanceName]bool
}

// NewAssetPushServer creates a gRPC service for serving the contents
// of a Remote Asset Push server.
func NewAssetPushServer(referenceStore *storage.ReferenceStore, allowUpdatesForInstances map[digest.InstanceName]bool) remoteasset.PushServer {
	return &assetPushServer{
		referenceStore:           referenceStore,
		allowUpdatesForInstances: allowUpdatesForInstances,
	}
}

func (s *assetPushServer) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	if !s.allowUpdatesForInstances[instanceName] {
		return nil, status.Errorf(codes.PermissionDenied, "This service does not accept Blobs for instance %#v", req.InstanceName)
	}

	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		err = s.referenceStore.Put(ctx, assetRef, req.BlobDigest, instanceName)
		if err != nil {
			return nil, err
		}
	}

	return &remoteasset.PushBlobResponse{}, nil
}

func (s *assetPushServer) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	instanceName, err := digest.NewInstanceName(req.InstanceName)
	if err != nil {
		return nil, util.StatusWrapf(err, "Invalid instance name %#v", req.InstanceName)
	}

	if !s.allowUpdatesForInstances[instanceName] {
		return nil, status.Errorf(codes.PermissionDenied, "This service does not accept Directories for instance %#v", req.InstanceName)
	}

	for _, uri := range req.Uris {
		assetRef := storage.NewAssetReference(uri, req.Qualifiers)
		err = s.referenceStore.Put(ctx, assetRef, req.RootDirectoryDigest, instanceName)
		if err != nil {
			return nil, err
		}
	}

	return &remoteasset.PushDirectoryResponse{}, nil
}
