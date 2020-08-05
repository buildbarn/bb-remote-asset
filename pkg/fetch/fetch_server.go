package fetch

import (
	"context"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/digest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetFetchServer struct {
	referenceStore           *storage.ReferenceStore
	allowUpdatesForInstances map[digest.InstanceName]bool
}

// NewAssetFetchServer creates a gRPC service for serving the contents
// of a Remote Asset Fetch server.
func NewAssetFetchServer(referenceStore *storage.ReferenceStore, allowUpdatesForInstances map[digest.InstanceName]bool) remoteasset.FetchServer {
	return &assetFetchServer{
		referenceStore:           referenceStore,
		allowUpdatesForInstances: allowUpdatesForInstances,
	}
}

func (s *assetFetchServer) FetchBlob(ctx context.Context, req *remoteasset.FetchBlobRequest) (*remoteasset.FetchBlobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "FetchBlob not implemented")
}

func (s *assetFetchServer) FetchDirectory(ctx context.Context, req *remoteasset.FetchDirectoryRequest) (*remoteasset.FetchDirectoryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "FetchDirectory not implemented")
}
