package push

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/digest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetPushServer struct {
	blobAccess               blobstore.BlobAccess
	allowUpdatesForInstances map[digest.InstanceName]bool
	maximumMessageSizeBytes  int
}

// NewAssetPushServer creates a gRPC service for serving the contents
// of a Remote Asset Push server.
func NewAssetPushServer(blobAccess blobstore.BlobAccess, allowUpdatesForInstances map[digest.InstanceName]bool, maximumMessageSizeBytes int) remoteasset.PushServer {
	return &assetPushServer{
		blobAccess:               blobAccess,
		allowUpdatesForInstances: allowUpdatesForInstances,
		maximumMessageSizeBytes:  maximumMessageSizeBytes,
	}
}

func (s *assetPushServer) PushBlob(ctx context.Context, req *remoteasset.PushBlobRequest) (*remoteasset.PushBlobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "PushBlob not implemented")
}

func (s *assetPushServer) PushDirectory(ctx context.Context, req *remoteasset.PushDirectoryRequest) (*remoteasset.PushDirectoryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "PushDirectory not implemented")
}
