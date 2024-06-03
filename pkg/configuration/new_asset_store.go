package configuration

import (
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	asset_configuration "github.com/buildbarn/bb-remote-asset/pkg/storage/blobstore"
	"github.com/buildbarn/bb-storage/pkg/auth"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/program"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewAssetStoreFromConfiguration creates an Asset Store from a
// configuration and CAS
func NewAssetStoreFromConfiguration(
	configuration *pb.AssetCacheConfiguration,
	contentAddressableStorage *blobstore_configuration.BlobAccessInfo,
	grpcClientFactory grpc.ClientFactory,
	maximumMessageSizeBytes int,
	dependenciesGroup program.Group,
	fetchAuthorizer auth.Authorizer,
	pushAuthorizer auth.Authorizer,
) (storage.AssetStore, error) {
	var assetStore storage.AssetStore
	switch backend := configuration.Backend.(type) {
	case *pb.AssetCacheConfiguration_BlobAccess:
		assetBlobAccessCreator := asset_configuration.NewAssetBlobAccessCreator(grpcClientFactory, maximumMessageSizeBytes)

		assetBlobAccess, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			dependenciesGroup,
			backend.BlobAccess,
			assetBlobAccessCreator)
		if err != nil {
			return nil, err
		}
		assetStore = storage.NewBlobAccessAssetStore(assetBlobAccess.BlobAccess, maximumMessageSizeBytes)
	case *pb.AssetCacheConfiguration_ActionCache:
		actionCache, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			dependenciesGroup,
			backend.ActionCache,
			blobstore_configuration.NewACBlobAccessCreator(
				contentAddressableStorage,
				grpcClientFactory,
				maximumMessageSizeBytes))
		if err != nil {
			return nil, err
		}
		assetStore = storage.NewActionCacheAssetStore(actionCache.BlobAccess, contentAddressableStorage.BlobAccess, maximumMessageSizeBytes)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Asset Cache configuration is invalid as no supported Asset Cache is defined.")
	}
	return storage.NewAuthorizingAssetStore(assetStore, fetchAuthorizer, pushAuthorizer), nil
}
