package blobstore

import (
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
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

func (brc assetBlobReplicatorCreator) NewCustomBlobReplicator(configuration *pb.BlobReplicatorConfiguration, source blobstore.BlobAccess, sink blobstore.BlobAccess) (replication.BlobReplicator, error) {
	return nil, status.Error(codes.InvalidArgument, "Configuration did not contain a supported replicator")
}

// AssetBlobReplicatorCreator is a BlobReplicatorCreator capable of creating
// BlobReplicators suitable for replicating Assets.
var AssetBlobReplicatorCreator configuration.BlobReplicatorCreator = assetBlobReplicatorCreator{}
