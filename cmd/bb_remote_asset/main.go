package main

import (
	"log"
	"net/http"
	"os"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/configuration"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	asset_configuration "github.com/buildbarn/bb-remote-asset/pkg/storage/blobstore"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/clock"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/gorilla/mux"

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
	if err := util.UnmarshalConfigurationFromFile(os.Args[1], &config); err != nil {
		log.Fatalf("Failed to read configuration from %s: %s", os.Args[1], err)
	}
	if err := global.ApplyConfiguration(config.Global); err != nil {
		log.Fatal("Failed to apply global configuration options: ", err)
	}

	// Initialize CAS storage access
	grpcClientFactory := bb_grpc.NewDeduplicatingClientFactory(bb_grpc.BaseClientFactory)
	casBlobAccessCreator := blobstore_configuration.NewCASBlobAccessCreator(grpcClientFactory, int(config.MaximumMessageSizeBytes))
	assetBlobAccessCreator := asset_configuration.NewAssetBlobAccessCreator(grpcClientFactory, int(config.MaximumMessageSizeBytes))

	assetBlobAccess, err := blobstore_configuration.NewBlobAccessFromConfiguration(
		config.AssetStore,
		assetBlobAccessCreator)
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}
	assetStore := storage.NewAssetStore(assetBlobAccess, int(config.MaximumMessageSizeBytes))

	allowUpdatesForInstances := map[bb_digest.InstanceName]bool{}
	for _, instance := range config.AllowUpdatesForInstances {
		instanceName, err := bb_digest.NewInstanceName(instance)
		if err != nil {
			log.Fatalf("Invalid instance name %#v: %s", instance, err)
		}
		allowUpdatesForInstances[instanceName] = true
	}

	fetchServer, err := configuration.NewFetcherFromConfiguration(config.Fetcher, assetStore, casBlobAccessCreator)
	if err != nil {
		log.Fatal("Failed to initialize fetch server from configuration: ", err)
	}

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

	// Web server for metrics and profiling.
	router := mux.NewRouter()
	util.RegisterAdministrativeHTTPEndpoints(router)
	log.Fatal(http.ListenAndServe(config.HttpListenAddress, router))
}
