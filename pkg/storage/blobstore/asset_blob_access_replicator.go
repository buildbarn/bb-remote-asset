package blobstore

import (
	"sync"

	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/blobstore/local"
	"github.com/buildbarn/bb-storage/pkg/blobstore/replication"
	"github.com/buildbarn/bb-storage/pkg/digest"
	pb "github.com/buildbarn/bb-storage/pkg/proto/configuration/blobstore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type assetBlobReplicatorCreator struct{}

func (brc assetBlobReplicatorCreator) GetDigestKeyFormat() digest.KeyFormat {
	return digest.KeyWithInstance
}

func (brc assetBlobReplicatorCreator) NewCustomBlobReplicator(configuration *pb.BlobReplicatorConfiguration, source blobstore.BlobAccess, sink configuration.BlobAccessInfo) (replication.BlobReplicator, error) {
	return nil, status.Error(codes.InvalidArgument, "Configuration did not contain a supported replicator")
}

func (brc *assetBlobReplicatorCreator) NewBlockListGrowthPolicy(currentBlocks, newBlocks int) (local.BlockListGrowthPolicy, error) {
	if newBlocks != 1 {
		return nil, status.Error(codes.InvalidArgument, "The number of \"new\" blocks must be set to 1 for this storage type, as objects cannot be updated reliably otherwise")
	}
	return local.NewMutableBlockListGrowthPolicy(currentBlocks), nil
}

func (brc *assetBlobReplicatorCreator) NewHierarchicalInstanceNamesLocalBlobAccess(keyLocationMap local.KeyLocationMap, locationBlobMap local.LocationBlobMap, globalLock *sync.RWMutex) (blobstore.BlobAccess, error) {
	return nil, status.Error(codes.InvalidArgument, "The hierarchical instance names option can only be used for the Content Addressable Storage")
}

// AssetBlobReplicatorCreator is a BlobReplicatorCreator capable of creating
// BlobReplicators suitable for replicating Assets.
var AssetBlobReplicatorCreator configuration.BlobReplicatorCreator = assetBlobReplicatorCreator{}
