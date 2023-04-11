package configuration

import (
	"log"
	"net/http"

	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/grpc"
	bb_http "github.com/buildbarn/bb-storage/pkg/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewFetcherFromConfiguration creates a new Remote Asset API Fetch
// server from a jsonnet configuration.
func NewFetcherFromConfiguration(configuration *pb.FetcherConfiguration,
	assetStore storage.AssetStore,
	contentAddressableStorage blobstore.BlobAccess,
	grpcClientFactory grpc.ClientFactory,
	maximumMessageSizeBytes int) (fetch.Fetcher, error) {
	var fetcher fetch.Fetcher
	switch backend := configuration.Backend.(type) {
	case *pb.FetcherConfiguration_Caching:
		innerFetcher, err := NewFetcherFromConfiguration(backend.Caching.Fetcher, assetStore, contentAddressableStorage, grpcClientFactory, maximumMessageSizeBytes)
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
		roundTripper, err := bb_http.NewRoundTripperFromConfiguration(backend.Http.Client)
		if err != nil {
			return nil, err
		}
		fetcher = fetch.NewHTTPFetcher(
			&http.Client{Transport: roundTripper},
			contentAddressableStorage,
			allowUpdatesForInstances)
	case *pb.FetcherConfiguration_Error:
		fetcher = fetch.NewErrorFetcher(backend.Error)
	case *pb.FetcherConfiguration_RemoteExecution:
		client, err := grpcClientFactory.NewClientFromConfiguration(backend.RemoteExecution.ExecutionClient)
		if err != nil {
			return nil, err
		}
		fetcher = fetch.NewRemoteExecutionFetcher(contentAddressableStorage, client, maximumMessageSizeBytes)
	default:
		return nil, status.Errorf(codes.InvalidArgument, "Fetcher configuration is invalid as no supported Fetchers are defined.")
	}

	return fetcher, nil
}
