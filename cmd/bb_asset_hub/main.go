package main

import (
	"log"
	"net/http"
	"os"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-asset-hub/pkg/fetch"
	"github.com/buildbarn/bb-asset-hub/pkg/proto/configuration/bb_asset_hub"
	"github.com/buildbarn/bb-asset-hub/pkg/push"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	asset_configuration "github.com/buildbarn/bb-asset-hub/pkg/storage/blobstore"
	blobstore_configuration "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/global"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/gorilla/mux"

	"google.golang.org/grpc"
)

const (
	// rfc3339Milli is identical similar to the time.RFC3339 and
	// time.RFC3339Nano formats, except that it shows the time in
	// milliseconds.
	rfc3339Milli = "2006-01-02T15:04:05.999Z07:00"
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
		log.Fatal("Usage: bb_asset_hub bb_asset_hub.jsonnet")
	}
	var config bb_asset_hub.ApplicationConfiguration
	if err := util.UnmarshalConfigurationFromFile(os.Args[1], &config); err != nil {
		log.Fatalf("Failed to read configuration from %s: %s", os.Args[1], err)
	}
	if err := global.ApplyConfiguration(config.Global); err != nil {
		log.Fatal("Failed to apply global configuration options: ", err)
	}

	// Initialize CAS storage access
	grpcClientFactory := bb_grpc.NewDeduplicatingClientFactory(bb_grpc.BaseClientFactory)
	casBlobAccessCreator := blobstore_configuration.NewCASBlobAccessCreator(grpcClientFactory, int(config.MaximumMessageSizeBytes))
	contentAddressableStorageBlobAccess, err := blobstore_configuration.NewBlobAccessFromConfiguration(
		config.ContentAddressableStorage,
		casBlobAccessCreator)
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}

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

	// TODO: Build configuration layer for fetchers
	fetchServer := fetch.NewCachingFetcher(
		fetch.NewHTTPFetcher(http.DefaultClient, contentAddressableStorageBlobAccess),
		assetStore)

	pushServer := push.NewAssetPushServer(
		assetStore,
		allowUpdatesForInstances)

	// Spawn gRPC servers for client and worker traffic.
	go func() {
		log.Fatal(
			"Client gRPC server failure: ",
			bb_grpc.NewServersFromConfigurationAndServe(
				config.GrpcServers,
				func(s *grpc.Server) {
					// Register services
					remoteasset.RegisterFetchServer(s, fetchServer)
					remoteasset.RegisterPushServer(s, pushServer)
				}))
	}()

	// Web server for metrics and profiling.
	router := mux.NewRouter()
	util.RegisterAdministrativeHTTPEndpoints(router)
	log.Fatal(http.ListenAndServe(config.HttpListenAddress, router))
}
