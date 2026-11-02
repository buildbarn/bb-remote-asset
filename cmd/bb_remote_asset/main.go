package main

import (
	"context"
	"os"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/pkg/configuration"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	auth_configuration "github.com/buildbarn/bb-storage/pkg/auth/configuration"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	bb_cas "github.com/buildbarn/bb-storage/pkg/cas"
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
		lifecycleState, grpcClientFactory, err := global.ApplyConfiguration(config.Global, dependenciesGroup)
		if err != nil {
			return util.StatusWrap(err, "Failed to apply global configuration options")
		}

		fetchAuthorizer, err := auth_configuration.DefaultAuthorizerFactory.NewAuthorizerFromConfiguration(config.FetchAuthorizer, dependenciesGroup, grpcClientFactory)
		if err != nil {
			return util.StatusWrap(err, "Failed to create Fetch Authorizer from Configuration")
		}

		pushAuthorizer, err := auth_configuration.DefaultAuthorizerFactory.NewAuthorizerFromConfiguration(config.PushAuthorizer, dependenciesGroup, grpcClientFactory)
		if err != nil {
			return util.StatusWrap(err, "Failed to create Push Authorizer from Configuration")
		}

		// Initialize CAS storage access.
		casInfo, err := blobstore_configuration.NewCASFromConfiguration(
			dependenciesGroup,
			config.ContentAddressableStorage,
			grpcClientFactory,
			int(config.MaximumMessageSizeBytes),
		)
		if err != nil {
			return util.StatusWrap(err, "Failed to create CAS")
		}
		readerPutter := bb_cas.NewReaderPutter(casInfo.ChunkStorage, casInfo.ChunkMappingStorage)
		directoryReader := bb_cas.NewMessageReader[remoteexecution.Directory](
			casInfo.ReadInfo,
			int(config.MaximumMessageSizeBytes),
		)
		treeReader := bb_cas.NewStorageBackedStreamReader(casInfo.ReadInfo)

		var assetStore storage.AssetStore
		if config.AssetCache != nil {
			var actionCache blobstore_configuration.BlobAccessInfo[*remoteexecution.ActionResult]
			if config.AssetCache.GetActionCache() != nil {
				actionCache, err = blobstore_configuration.NewBlobAccessFromConfiguration(
					dependenciesGroup,
					config.AssetCache.GetActionCache(),
					blobstore_configuration.NewACBlobAccessCreator(
						&casInfo,
						grpcClientFactory,
						int(config.MaximumMessageSizeBytes),
					),
				)
				if err != nil {
					return util.StatusWrap(err, "Failed to create Action Cache")
				}
			}
			assetStore, err = configuration.NewAssetStoreFromConfiguration(
				config.AssetCache,
				actionCache,
				readerPutter,
				directoryReader,
				casInfo.CDCParametersFetcher,
				grpcClientFactory,
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
			readerPutter,
			treeReader,
			casInfo.CDCParametersFetcher,
			grpcClientFactory,
			dependenciesGroup,
			fetchAuthorizer,
		)
		if err != nil {
			return util.StatusWrap(err, "Failed to initialize fetch server from configuration")
		}

		var metricsPushServer remoteasset.PushServer
		if assetStore != nil {
			pushServer := push.NewAssetPushServer(
				assetStore,
				allowUpdatesForInstances,
			)
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
			grpcClientFactory,
		); err != nil {
			return util.StatusWrap(err, "gRPC server failure")
		}

		lifecycleState.MarkReadyAndWait(siblingsGroup)
		return nil
	})
}
