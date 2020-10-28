package configuration

import (
	"log"
	"net/http"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset/fetch"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewFetcherFromConfiguration creates a new Remote Asset API Fetch
// server from a jsonnet configuration.
func NewFetcherFromConfiguration(configuration *pb.FetcherConfiguration,
	contentAddressableStorage, actionCache blobstore.BlobAccess,
	pushServer remoteasset.PushServer, maximumSizeBytes int) (fetch.Fetcher, error) {
	var fetcher fetch.Fetcher
	switch backend := configuration.Backend.(type) {
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
		fetcher = fetch.NewHTTPFetcher(
			http.DefaultClient,
			contentAddressableStorage,
			allowUpdatesForInstances)
	case *pb.FetcherConfiguration_Error:
		fetcher = fetch.NewErrorFetcher(backend.Error)
	case *pb.FetcherConfiguration_ActionCache:
		innerFetcher, err := NewFetcherFromConfiguration(backend.ActionCache.Fetcher, contentAddressableStorage, actionCache, pushServer, maximumSizeBytes)
		if err != nil {
			return nil, err
		}
		fetcher = fetch.NewActionCachingFetcher(innerFetcher, pushServer, actionCache, contentAddressableStorage, maximumSizeBytes)

	default:
		return nil, status.Errorf(codes.InvalidArgument, "Fetcher configuration is invalid as no supported Fetchers are defined.")
	}

	return fetch.NewValidatingFetcher(fetch.NewLoggingFetcher(fetcher)), nil
}
