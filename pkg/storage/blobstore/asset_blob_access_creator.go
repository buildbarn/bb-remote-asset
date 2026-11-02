package blobstore

import (
	"sync"

	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/coder"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/blobstore/local"
	"github.com/buildbarn/bb-storage/pkg/capabilities"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/program"
	pb "github.com/buildbarn/bb-storage/pkg/proto/configuration/blobstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetBlobAccessCreator struct {
	assetBlobReplicatorCreator

	grpcClientFactory grpc.ClientFactory
}

// NewAssetBlobAccessCreator creates a new BlobAccessCreator suitable for creating BlobAccesses
// used for storage of Assets.
func NewAssetBlobAccessCreator(grpcClientFactory grpc.ClientFactory) configuration.BlobAccessCreator[*asset.Asset] {
	return &assetBlobAccessCreator{
		grpcClientFactory: grpcClientFactory,
	}
}

func (bac *assetBlobAccessCreator) GetBaseDigestKeyFormat() digest.KeyFormat {
	return digest.KeyWithInstance
}

func (bac *assetBlobAccessCreator) GetStorageTypeName() string {
	return "asset"
}

func (bac *assetBlobAccessCreator) GetDefaultCapabilitiesProvider() capabilities.Provider {
	return capabilities.NewStaticProvider(&remoteexecution.ServerCapabilities{
		// TODO: what are the capabilities?
	})
}

func (bac *assetBlobAccessCreator) GetBinaryCoder() coder.Coder[*asset.Asset, []byte] {
	c := coder.NewProtoCoder[asset.Asset]()
	return coder.JoinCoders(c, coder.NewSHA256SuffixCoder())
}

func (bac *assetBlobAccessCreator) NewBlockListGrowthPolicy(currentBlocks, newBlocks int) (local.BlockListGrowthPolicy, error) {
	if newBlocks != 1 {
		return nil, status.Error(codes.InvalidArgument, "The number of \"new\" blocks must be set to 1 for this storage type, as objects cannot be updated reliably otherwise")
	}
	return local.NewMutableBlockListGrowthPolicy(currentBlocks), nil
}

func (bac *assetBlobAccessCreator) NewCustomBlobAccess(terminationGroup program.Group, config *pb.BlobAccessConfiguration, nestedCreator configuration.NestedBlobAccessCreator[*asset.Asset]) (configuration.BlobAccessInfo[*asset.Asset], string, error) {
	return configuration.BlobAccessInfo[*asset.Asset]{}, "", status.Error(codes.InvalidArgument, "Configuration did not contain a supported storage backend")
}

func (bac *assetBlobAccessCreator) WrapTopLevelBlobAccess(blobAccess blobstore.BlobAccess[*asset.Asset]) blobstore.BlobAccess[*asset.Asset] {
	return blobAccess
}

func (bac *assetBlobAccessCreator) NewHierarchicalInstanceNamesLocalBlobAccess(keyLocationMap local.KeyLocationMap, blockReferenceResolver local.BlockReferenceResolver, locationBlobMap local.LocationBlobMap, globalLock *sync.RWMutex, capabilitiesProvider capabilities.Provider) (blobstore.BlobAccess[*asset.Asset], error) {
	return nil, status.Error(codes.Unimplemented, "NewHierarchicalInstanceNamesLocalBlobAccess unimplemeted for assetBlobAccessCreator")
}
