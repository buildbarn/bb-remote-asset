package main

import (
	"log"
	"os"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/configuration"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/clock"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/util"
	protostatus "google.golang.org/genproto/googleapis/rpc/status"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// timestampDelta is returned by the timestamp_proto_delta, returning a
// timestamp and a duration relative to a previous timestamp value. It
// can be used to display split times.
type timestampDelta struct {
	Time                 time.Time
	DurationFromPrevious time.Duration
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: bb_remote_asset bb_remote_asset.jsonnet")
	}
	var config bb_remote_asset.ApplicationConfiguration
	var err error
	if err := util.UnmarshalConfigurationFromFile(os.Args[1], &config); err != nil {
		log.Fatalf("Failed to read configuration from %s: %s", os.Args[1], err)
	}
	lifecycleState, err := global.ApplyConfiguration(config.Global)
	if err != nil {
		log.Fatal("Failed to apply global configuration options: ", err)
	}

	// Initialize CAS storage access
	grpcClientFactory := bb_grpc.DefaultClientFactory

	contentAddressableStorageInfo, err := blobstore_configuration.NewBlobAccessFromConfiguration(config.ContentAddressableStorage, blobstore_configuration.NewCASBlobAccessCreator(grpcClientFactory, int(config.MaximumMessageSizeBytes)))
	if err != nil {
		log.Fatalf("Failed to create CAS blob access: %v", err)
	}

	var assetStore storage.AssetStore
	if config.AssetCache != nil {
		assetStore, err = configuration.NewAssetStoreFromConfiguration(config.AssetCache, contentAddressableStorageInfo, grpcClientFactory, int(config.MaximumMessageSizeBytes))
		if err != nil {
			log.Fatalf("Failed to create asset store: %v", err)
		}
	}

	allowUpdatesForInstances := map[bb_digest.InstanceName]bool{}
	for _, instance := range config.AllowUpdatesForInstances {
		instanceName, err := bb_digest.NewInstanceName(instance)
		if err != nil {
			log.Fatalf("Invalid instance name %#v: %s", instance, err)
		}
		allowUpdatesForInstances[instanceName] = true
	}

	fetchServer, err := configuration.NewFetcherFromConfiguration(config.Fetcher, assetStore, contentAddressableStorageInfo.BlobAccess, grpcClientFactory, int(config.MaximumMessageSizeBytes))
	if err != nil {
		log.Fatal("Failed to initialize fetch server from configuration: ", err)
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
	go func() {
		log.Fatal(
			"Client gRPC server failure: ",
			bb_grpc.NewServersFromConfigurationAndServe(
				config.GrpcServers,
				func(s *grpc.Server) {
					// Register services
					remoteasset.RegisterFetchServer(s, fetchServer)
					remoteasset.RegisterPushServer(s, metricsPushServer)
				}))
	}()

	lifecycleState.MarkReadyAndWait()
}
