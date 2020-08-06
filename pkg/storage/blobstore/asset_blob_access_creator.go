package blobstore

import (
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/grpc"
	pb "github.com/buildbarn/bb-storage/pkg/proto/configuration/blobstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetBlobAccessCreator struct {
	assetBlobReplicatorCreator

	grpcClientFactory       grpc.ClientFactory
	maximumMessageSizeBytes int
}

// NewAssetBlobAccessCreator creates a new BlobAccessCreator suitable for creating BlobAccesses
// used for storage of Assets.
func NewAssetBlobAccessCreator(grpcClientFactory grpc.ClientFactory, maximumMessageSizeBytes int) configuration.BlobAccessCreator {
	return &assetBlobAccessCreator{
		grpcClientFactory:       grpcClientFactory,
		maximumMessageSizeBytes: maximumMessageSizeBytes,
	}
}

func (bac *assetBlobAccessCreator) GetStorageType() blobstore.StorageType {
	return AssetStorageType
}

func (bac *assetBlobAccessCreator) GetStorageTypeName() string {
	return "asset"
}

func (bac *assetBlobAccessCreator) NewCustomBlobAccess(configuration *pb.BlobAccessConfiguration) (blobstore.BlobAccess, string, error) {
	return nil, "", status.Error(codes.InvalidArgument, "Configuration did not contain a supported storage backend")
}

func (bac *assetBlobAccessCreator) WrapTopLevelBlobAccess(blobAccess blobstore.BlobAccess) blobstore.BlobAccess {
	return blobAccess
}
