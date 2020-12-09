package main

import (
	"log"
	"os"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/configuration"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-storage/pkg/clock"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/util"

	"google.golang.org/grpc"
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

	assetStore, contentAddressableStorage, err := configuration.NewAssetStoreAndCASFromConfiguration(config.AssetCache, grpcClientFactory, int(config.MaximumMessageSizeBytes))
	if err != nil {
		log.Fatalf("Failed to create asset store and CAS: %v", err)
	}

	allowUpdatesForInstances := map[bb_digest.InstanceName]bool{}
	for _, instance := range config.AllowUpdatesForInstances {
		instanceName, err := bb_digest.NewInstanceName(instance)
		if err != nil {
			log.Fatalf("Invalid instance name %#v: %s", instance, err)
		}
		allowUpdatesForInstances[instanceName] = true
	}

	fetchServer, err := configuration.NewFetcherFromConfiguration(config.Fetcher, assetStore, contentAddressableStorage, grpcClientFactory, int(config.MaximumMessageSizeBytes))
	if err != nil {
		log.Fatal("Failed to initialize fetch server from configuration: ", err)
	}
	fetchServer = fetch.NewMetricsFetcher(
		fetch.NewValidatingFetcher(
			fetch.NewLoggingFetcher(fetchServer),
		),
		clock.SystemClock, "fetch",
	)

	pushServer := push.NewAssetPushServer(
		assetStore,
		allowUpdatesForInstances)
	metricsPushServer := push.NewMetricsAssetPushServer(pushServer, clock.SystemClock, "push")

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
