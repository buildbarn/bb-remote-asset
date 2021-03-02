package configuration

import (
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	asset_configuration "github.com/buildbarn/bb-remote-asset/pkg/storage/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewAssetStoreAndCASFromConfiguration creates an Asset Store and
// BlobAccess for the Content Addressable Storage.
func NewAssetStoreAndCASFromConfiguration(configuration *pb.AssetCacheConfiguration, grpcClientFactory grpc.ClientFactory, maximumMessageSizeBytes int) (storage.AssetStore, blobstore.BlobAccess, error) {
	var assetStore storage.AssetStore
	var contentAddressableStorage blobstore.BlobAccess
	switch backend := configuration.Backend.(type) {
	case *pb.AssetCacheConfiguration_BlobAccess:
		assetBlobAccessCreator := asset_configuration.NewAssetBlobAccessCreator(grpcClientFactory, maximumMessageSizeBytes)

		assetBlobAccess, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			backend.BlobAccess.AssetStore,
			assetBlobAccessCreator)
		if err != nil {
			return nil, nil, err
		}
		assetStore = storage.NewBlobAccessAssetStore(assetBlobAccess.BlobAccess, maximumMessageSizeBytes)
		contentAddressableStorageInfo, err := blobstore_configuration.NewBlobAccessFromConfiguration(backend.BlobAccess.ContentAddressableStorage, blobstore_configuration.NewCASBlobAccessCreator(grpcClientFactory, maximumMessageSizeBytes))
		if err != nil {
			return nil, nil, err
		}
		contentAddressableStorage = contentAddressableStorageInfo.BlobAccess
	case *pb.AssetCacheConfiguration_ActionCache:
		cas, actionCache, err := blobstore_configuration.NewCASAndACBlobAccessFromConfiguration(backend.ActionCache.Blobstore, grpcClientFactory, maximumMessageSizeBytes)
		if err != nil {
			return nil, nil, err
		}
		contentAddressableStorage = cas
		assetStore = storage.NewActionCacheAssetStore(actionCache, contentAddressableStorage, maximumMessageSizeBytes)
	case *pb.AssetCacheConfiguration_CasOnly:
		contentAddressableStorageInfo, err := blobstore_configuration.NewBlobAccessFromConfiguration(backend.CasOnly.ContentAddressableStorage, blobstore_configuration.NewCASBlobAccessCreator(grpcClientFactory, maximumMessageSizeBytes))
		if err != nil {
			return nil, nil, err
		}
		contentAddressableStorage = contentAddressableStorageInfo.BlobAccess
	default:
		return nil, nil, status.Errorf(codes.InvalidArgument, "Asset Cache configuration is invalid as no supported Asset Cache is defined.")
	}
	return assetStore, contentAddressableStorage, nil
}
