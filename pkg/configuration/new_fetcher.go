package configuration

import (
	"net/http"

	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/auth"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/clock"
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
	maximumMessageSizeBytes int,
	authorizer auth.Authorizer,
) (fetch.Fetcher, error) {
	var fetcher fetch.Fetcher
	if configuration == nil {
		fetcher = fetch.DefaultFetcher
	} else {
		switch backend := configuration.Backend.(type) {
		case *pb.FetcherConfiguration_Http:
			roundTripper, err := bb_http.NewRoundTripperFromConfiguration(backend.Http.Client)
			if err != nil {
				return nil, err
			}
			fetcher = fetch.NewHTTPFetcher(
				&http.Client{Transport: roundTripper},
				contentAddressableStorage)
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
	}
	if assetStore != nil {
		fetcher = fetch.NewCachingFetcher(fetcher, assetStore)
	}
	return fetch.NewAuthorizingFetcher(
		fetch.NewMetricsFetcher(
			fetch.NewLoggingFetcher(
				fetch.NewValidatingFetcher(fetcher),
			),
			clock.SystemClock,
			"fetch",
		),
		authorizer,
	), nil
}
