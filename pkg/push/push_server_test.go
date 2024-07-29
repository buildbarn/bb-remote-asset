package push_test

import (
	"context"
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestPushServerPushBlobSuccess(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)
	blobDigest := &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123}
	uri := "https://example.com/example.txt"
	qualifiers := []*remoteasset.Qualifier{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "biff",
			Value: "boff",
		},
	}
	request := &remoteasset.PushBlobRequest{
		InstanceName: "",
		Uris:         []string{uri},
		BlobDigest:   blobDigest,
		Qualifiers:   qualifiers,
	}
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference([]string{uri}, qualifiers), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	backend.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&asset.Asset{}, 1000)
			require.NoError(t, err)
			a := m.(*asset.Asset)
			require.True(t, proto.Equal(a.Digest, blobDigest))
			require.Equal(t, asset.Asset_BLOB, a.Type)
			return nil
		})
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	pushServer := push.NewAssetPushServer(assetStore, map[digest.InstanceName]bool{instanceName: true})

	response, err := pushServer.PushBlob(ctx, request)
	require.NoError(t, err)
	require.Equal(t, &remoteasset.PushBlobResponse{}, response)
}

func TestPushServerPushDirectorySuccess(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)
	rootDirectoryDigest := &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123}
	uri := "https://example.com/example.txt"
	qualifiers := []*remoteasset.Qualifier{
		{
			Name:  "resource_type",
			Value: "application/x-git",
		},
	}
	request := &remoteasset.PushDirectoryRequest{
		InstanceName:        "",
		Uris:                []string{uri},
		RootDirectoryDigest: rootDirectoryDigest,
		Qualifiers:          qualifiers,
	}
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference([]string{uri}, qualifiers), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	backend.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&asset.Asset{}, 1000)
			require.NoError(t, err)
			a := m.(*asset.Asset)
			require.True(t, proto.Equal(a.Digest, rootDirectoryDigest))
			require.Equal(t, asset.Asset_DIRECTORY, a.Type)
			return nil
		})
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	pushServer := push.NewAssetPushServer(assetStore, map[digest.InstanceName]bool{instanceName: true})

	response, err := pushServer.PushDirectory(ctx, request)
	require.NoError(t, err)
	require.Equal(t, &remoteasset.PushDirectoryResponse{}, response)
}

func TestPushServerInvalidArgumentFailure(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	blobRequest := &remoteasset.PushBlobRequest{
		InstanceName: "",
	}
	directoryRequest := &remoteasset.PushDirectoryRequest{
		InstanceName: "",
	}

	backend := mock.NewMockBlobAccess(ctrl)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	pushServer := push.NewAssetPushServer(assetStore, map[digest.InstanceName]bool{instanceName: true})

	_, err = pushServer.PushBlob(ctx, blobRequest)
	require.Equal(t, status.Error(codes.InvalidArgument, "PushBlob requires at least one URI"), err)
	_, err = pushServer.PushDirectory(ctx, directoryRequest)
	require.Equal(t, status.Error(codes.InvalidArgument, "PushDirectory requires at least one URI"), err)
}

func TestPushServerBadInstanceName(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("good")
	require.NoError(t, err)

	blobRequest := &remoteasset.PushBlobRequest{
		InstanceName: "bad",
		Uris:         []string{"https://example.com/example.txt"},
		BlobDigest: &remoteexecution.Digest{
			Hash:      "2692b9fd6c5b85d5dfa4e6d1ab445c77d00a91fc23ab760ba7a75d81b8b7f685",
			SizeBytes: 123,
		},
	}
	directoryRequest := &remoteasset.PushDirectoryRequest{
		InstanceName: "bad",
		Uris:         []string{"https://example.com/example"},
		RootDirectoryDigest: &remoteexecution.Digest{
			Hash:      "6b6e188ba6c0db153b03eaf1bc353dd6bf159eba926d3cf68d6adb69112e8c3a",
			SizeBytes: 234,
		},
	}

	backend := mock.NewMockBlobAccess(ctrl)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	pushServer := push.NewAssetPushServer(assetStore, map[digest.InstanceName]bool{instanceName: true})

	_, err = pushServer.PushBlob(ctx, blobRequest)
	require.Equal(t, status.Error(codes.PermissionDenied, "This service does not accept Blobs for instance \"bad\""), err)
	_, err = pushServer.PushDirectory(ctx, directoryRequest)
	require.Equal(t, status.Error(codes.PermissionDenied, "This service does not accept Directories for instance \"bad\""), err)
}
