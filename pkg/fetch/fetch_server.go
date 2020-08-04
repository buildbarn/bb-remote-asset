package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/digest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetFetchServer struct {
	blobAccess               blobstore.BlobAccess
	allowUpdatesForInstances map[digest.InstanceName]bool
	maximumMessageSizeBytes  int
}

// NewAssetFetchServer creates a gRPC service for serving the contents
// of a Remote Asset Fetch server.
func NewAssetFetchServer(blobAccess blobstore.BlobAccess, allowUpdatesForInstances map[digest.InstanceName]bool, maximumMessageSizeBytes int) remoteasset.FetchServer {
	return &assetFetchServer{
		blobAccess:               blobAccess,
		allowUpdatesForInstances: allowUpdatesForInstances,
		maximumMessageSizeBytes:  maximumMessageSizeBytes,
	}
}

func (s *assetFetchServer) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "FetchBlob not implemented")
}

func (s *assetFetchServer) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "FetchDirectory not implemented")
}
