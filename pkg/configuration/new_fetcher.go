package configuration

import (
	"log"
	"net/http"

	"github.com/buildbarn/bb-asset-hub/pkg/fetch"
	pb "github.com/buildbarn/bb-asset-hub/pkg/proto/configuration/bb_asset_hub/fetch"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewFetcherFromConfiguration creates a new Remote Asset API Fetch server from
// a jsonnet configuration.
func NewFetcherFromConfiguration(configuration *pb.FetcherConfiguration,
	assetStore *storage.AssetStore,
	casBlobAccessCreator blobstore_configuration.BlobAccessCreator) (remoteasset.FetchServer, error) {
	var fetcher remoteasset.FetchServer
	switch backend := configuration.Backend.(type) {
	case *pb.FetcherConfiguration_Caching:
		innerFetcher, err := NewFetcherFromConfiguration(backend.Caching.Fetcher, assetStore, casBlobAccessCreator)
		if err != nil {
			return nil, err
		}
		fetcher = fetch.NewCachingFetcher(
			innerFetcher,
			assetStore)
	case *pb.FetcherConfiguration_Http:
		// TODO: Shift into utils lib as also used in main.go
		allowUpdatesForInstances := map[bb_digest.InstanceName]bool{}
		for _, instance := range backend.Http.AllowUpdatesForInstances {
			instanceName, err := bb_digest.NewInstanceName(instance)
			if err != nil {
				log.Fatalf("Invalid instance name %#v: %s", instance, err)
			}
			allowUpdatesForInstances[instanceName] = true
		}
		cas, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			backend.Http.ContentAddressableStorage,
			casBlobAccessCreator)
		if err != nil {
			return nil, err
		}
		fetcher = fetch.NewHTTPFetcher(
			http.DefaultClient,
			cas,
			allowUpdatesForInstances)
	case *pb.FetcherConfiguration_Error:
		fetcher = fetch.NewErrorFetcher(backend.Error)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Fetcher configuration is invalid as no supported Fetchers are defined.")
	}

	return fetch.NewValidatingFetcher(fetch.NewLoggingFetcher(fetcher)), nil
}
