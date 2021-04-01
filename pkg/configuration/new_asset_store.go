package configuration

import (
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	asset_configuration "github.com/buildbarn/bb-remote-asset/pkg/storage/blobstore"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewAssetStoreFromConfiguration creates an Asset Store from a
// configuration and CAS
func NewAssetStoreFromConfiguration(configuration *pb.AssetCacheConfiguration, contentAddressableStorage blobstore_configuration.BlobAccessInfo, grpcClientFactory grpc.ClientFactory, maximumMessageSizeBytes int) (storage.AssetStore, error) {
	if configuration == nil {
		return nil, nil
	}
	switch backend := configuration.Backend.(type) {
	case *pb.AssetCacheConfiguration_BlobAccess:
		assetBlobAccessCreator := asset_configuration.NewAssetBlobAccessCreator(grpcClientFactory, maximumMessageSizeBytes)

		assetBlobAccess, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			backend.BlobAccess,
			assetBlobAccessCreator)
		if err != nil {
			return nil, err
		}
		return storage.NewBlobAccessAssetStore(assetBlobAccess.BlobAccess, maximumMessageSizeBytes), nil
	case *pb.AssetCacheConfiguration_ActionCache:
		actionCache, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			backend.ActionCache,
			blobstore_configuration.NewACBlobAccessCreator(
				contentAddressableStorage,
				grpcClientFactory,
				maximumMessageSizeBytes))
		if err != nil {
			return nil, err
		}
		return storage.NewActionCacheAssetStore(actionCache.BlobAccess, contentAddressableStorage.BlobAccess, maximumMessageSizeBytes), nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Asset Cache configuration is invalid as no supported Asset Cache is defined.")
	}
	return nil, status.Errorf(codes.Internal, "Something went wrong creating Asset Cache.")
}
