package blobstore

import (
	"sync"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/blobstore/local"
	"github.com/buildbarn/bb-storage/pkg/capabilities"
	"github.com/buildbarn/bb-storage/pkg/digest"
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

func (bac *assetBlobAccessCreator) GetBaseDigestKeyFormat() digest.KeyFormat {
	return digest.KeyWithInstance
}

func (bac *assetBlobAccessCreator) GetReadBufferFactory() blobstore.ReadBufferFactory {
	return AssetReadBufferFactory
}

func (bac *assetBlobAccessCreator) GetStorageTypeName() string {
	return "asset"
}

func (bac *assetBlobAccessCreator) NewCustomBlobAccess(config *pb.BlobAccessConfiguration, creator configuration.NestedBlobAccessCreator) (configuration.BlobAccessInfo, string, error) {
	return configuration.BlobAccessInfo{}, "", status.Error(codes.InvalidArgument, "Configuration did not contain a supported storage backend")
}

func (bac *assetBlobAccessCreator) WrapTopLevelBlobAccess(blobAccess blobstore.BlobAccess) blobstore.BlobAccess {
	return blobAccess
}

func (bac *assetBlobAccessCreator) GetDefaultCapabilitiesProvider() capabilities.Provider {
	return capabilities.NewStaticProvider(&remoteexecution.ServerCapabilities{
		// TODO: what are the capabilities?
	})
}

func (bac *assetBlobAccessCreator) NewBlockListGrowthPolicy(currentBlocks, newBlocks int) (local.BlockListGrowthPolicy, error) {
	if newBlocks != 1 {
		return nil, status.Error(codes.InvalidArgument, "The number of \"new\" blocks must be set to 1 for this storage type, as objects cannot be updated reliably otherwise")
	}
	return local.NewMutableBlockListGrowthPolicy(currentBlocks), nil
}

func (bac *assetBlobAccessCreator) NewHierarchicalInstanceNamesLocalBlobAccess(local.KeyLocationMap, local.LocationBlobMap, *sync.RWMutex) (blobstore.BlobAccess, error) {
	return nil, status.Error(codes.Unimplemented, "NewHierarchicalInstanceNamesLocalBlobAccess unimplemeted for assetBlobAccessCreator")
}
