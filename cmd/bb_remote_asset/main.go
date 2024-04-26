package main

import (
	"context"
	"os"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/configuration"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/auth"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/clock"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/program"
	"github.com/buildbarn/bb-storage/pkg/util"
	protostatus "google.golang.org/genproto/googleapis/rpc/status"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// timestampDelta is returned by the timestamp_proto_delta, returning a
// timestamp and a duration relative to a previous timestamp value. It
// can be used to display split times.
type timestampDelta struct {
	Time                 time.Time
	DurationFromPrevious time.Duration
}

func main() {
	program.RunMain(func(ctx context.Context, siblingsGroup, dependenciesGroup program.Group) error {
		if len(os.Args) != 2 {
			return status.Error(codes.InvalidArgument, "Usage: bb_remote_asset bb_remote_asset.jsonnet")
		}
		var config bb_remote_asset.ApplicationConfiguration
		var err error
		if err := util.UnmarshalConfigurationFromFile(os.Args[1], &config); err != nil {
			return util.StatusWrapf(err, "Failed to read configuration from %s", os.Args[1])
		}
		lifecycleState, grpcClientFactory, err := global.ApplyConfiguration(config.Global)
		if err != nil {
			return util.StatusWrap(err, "Failed to apply global configuration options")
		}

		fetchAuthorizer, err := auth.DefaultAuthorizerFactory.NewAuthorizerFromConfiguration(config.FetchAuthorizer)
		if err != nil {
			return util.StatusWrap(err, "Failed to create Fetch Authorizer from Configuration")
		}

		pushAuthorizer, err := auth.DefaultAuthorizerFactory.NewAuthorizerFromConfiguration(config.PushAuthorizer)
		if err != nil {
			return util.StatusWrap(err, "Failed to create Push Authorizer from Configuration")
		}

		// Initialize CAS storage access
		contentAddressableStorageInfo, err := blobstore_configuration.NewBlobAccessFromConfiguration(
			dependenciesGroup,
			config.ContentAddressableStorage,
			blobstore_configuration.NewCASBlobAccessCreator(
				grpcClientFactory,
				int(config.MaximumMessageSizeBytes),
			),
		)
		if err != nil {
			return util.StatusWrap(err, "Failed to create CAS blob access")
		}
		var assetStore storage.AssetStore
		if config.AssetCache != nil {
			assetStore, err = configuration.NewAssetStoreFromConfiguration(
				config.AssetCache,
				&contentAddressableStorageInfo,
				grpcClientFactory,
				int(config.MaximumMessageSizeBytes),
				dependenciesGroup,
				fetchAuthorizer,
				pushAuthorizer,
			)
			if err != nil {
				return util.StatusWrap(err, "Failed to create asset store")
			}
		}

		allowUpdatesForInstances := map[bb_digest.InstanceName]bool{}
		for _, instance := range config.AllowUpdatesForInstances {
			instanceName, err := bb_digest.NewInstanceName(instance)
			if err != nil {
				return util.StatusWrapf(err, "Invalid instance name %#v", instance)
			}
			allowUpdatesForInstances[instanceName] = true
		}

		fetchServer, err := configuration.NewFetcherFromConfiguration(
			config.Fetcher,
			assetStore,
			contentAddressableStorageInfo.BlobAccess,
			grpcClientFactory,
			int(config.MaximumMessageSizeBytes),
			fetchAuthorizer,
		)
		if err != nil {
			return util.StatusWrap(err, "Failed to initialize fetch server from configuration")
		}

		var metricsPushServer remoteasset.PushServer
		if assetStore != nil {
			pushServer := push.NewAssetPushServer(
				assetStore,
				allowUpdatesForInstances)
			metricsPushServer = push.NewMetricsAssetPushServer(pushServer, clock.SystemClock, "push")
		} else {
			metricsPushServer = push.NewErrorPushServer(&protostatus.Status{
				Code:    int32(codes.FailedPrecondition),
				Message: "Server is not configured to allow pushing assets",
			})
		}

		// Spawn gRPC servers for client and worker traffic.
		if err := bb_grpc.NewServersFromConfigurationAndServe(
			config.GrpcServers,
			func(s grpc.ServiceRegistrar) {
				// Register services
				remoteasset.RegisterFetchServer(s, fetchServer)
				remoteasset.RegisterPushServer(s, metricsPushServer)
			},
			siblingsGroup,
		); err != nil {
			return util.StatusWrap(err, "gRPC server failure")
		}

		lifecycleState.MarkReadyAndWait(siblingsGroup)
		return nil
	})
}
