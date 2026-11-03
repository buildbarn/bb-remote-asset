package configuration

import (
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	asset_configuration "github.com/buildbarn/bb-remote-asset/pkg/storage/blobstore"
	"github.com/buildbarn/bb-storage/pkg/auth"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/capabilities"
	bb_cas "github.com/buildbarn/bb-storage/pkg/cas"
	"github.com/buildbarn/bb-storage/pkg/cas/reader"
	"github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/program"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewAssetStoreFromConfiguration creates an Asset Store from a
// configuration and CAS
func NewAssetStoreFromConfiguration(
	configuration *pb.AssetCacheConfiguration,
	actionCache blobstore_configuration.BlobAccessInfo[*remoteexecution.ActionResult],
	readerPutter bb_cas.ReaderPutter,
	directoryReader reader.Reader[*remoteexecution.Directory],
	cdcParametersFetcher capabilities.CDCParametersFetcher,
	grpcClientFactory grpc.ClientFactory,
	dependenciesGroup program.Group,
	fetchAuthorizer auth.Authorizer,
	pushAuthorizer auth.Authorizer,
) (storage.AssetStore, error) {
	var assetStore storage.AssetStore
	switch backend := configuration.Backend.(type) {
	case *pb.AssetCacheConfiguration_BlobAccess:
		assetBlobAccess, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			dependenciesGroup,
			backend.BlobAccess,
			asset_configuration.NewAssetBlobAccessCreator(
				grpcClientFactory,
			),
		)
		if err != nil {
			return nil, err
		}
		assetStore = storage.NewBlobAccessAssetStore(assetBlobAccess.BlobAccess)
	case *pb.AssetCacheConfiguration_ActionCache:
		assetStore = storage.NewActionCacheAssetStore(
			actionCache.BlobAccess,
			readerPutter,
			directoryReader,
			cdcParametersFetcher,
			configuration.ActionCachePlatform,
		)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Asset Cache configuration is invalid as no supported Asset Cache is defined.")
	}
	return storage.NewAuthorizingAssetStore(assetStore, fetchAuthorizer, pushAuthorizer), nil
}
