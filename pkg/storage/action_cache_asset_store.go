package storage

import "github.com/buildbarn/bb-storage/pkg/blobstore"

type actionCacheAssetStore struct {
	actionCache             blobstore.BlobAccess
	maximumMessageSizeBytes int
}

func NewActionCacheAssetStore(actionCache blobstore.BlobAccess, maximumMessageSizeBytes int) AssetStore {
	return &actionCacheAssetStore{
		actionCache:             actionCache,
		maximumMessageSizeBytes: maximumMessageSizeBytes,
	}
}
