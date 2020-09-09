package configuration

import (
	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	pb "github.com/buildbarn/bb-remote-asset/pkg/proto/configuration/bb_remote_asset/push"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewPusherFromConfiguration creates a new Remote Asset API Fetch server from
// a jsonnet configuration.
func NewPusherFromConfiguration(configuration *pb.PusherConfiguration,
	assetStore *storage.AssetStore,
	grpcClientFactory bb_grpc.ClientFactory) (remoteasset.PushServer, error) {
	var pusher remoteasset.PushServer
	switch backend := configuration.Backend.(type) {
	case *pb.PusherConfiguration_Caching:
		innerPusher, err := NewPusherFromConfiguration(backend.Caching, assetStore, grpcClientFactory)
		if err != nil {
			return nil, err
		}
		pusher = push.NewLocalCachingPusher(
			innerPusher,
			assetStore)
	case *pb.PusherConfiguration_Error:
		pusher = push.NewErrorPusher(backend.Error)
	case *pb.PusherConfiguration_ActionCache:
		client, err := grpcClientFactory.NewClientFromConfiguration(backend.ActionCache.ActionCache)
		if err != nil {
			return nil, err
		}
		innerPusher, err := NewPusherFromConfiguration(backend.ActionCache.Pusher, assetStore, grpcClientFactory)
		if err != nil {
			return nil, err
		}
		pusher = push.NewActionCachingPusher(innerPusher, client)

	default:
		return nil, status.Errorf(codes.InvalidArgument, "Pusher configuration is invalid as no supported Pushers are defined.")
	}

	return push.NewValidatingPusher(push.NewLoggingPusher(pusher)), nil
}
