package fetch_test

import (
	"context"
	"testing"

	"github.com/buildbarn/bb-asset-hub/internal/mock"
	"github.com/buildbarn/bb-asset-hub/pkg/fetch"
	"github.com/buildbarn/bb-asset-hub/pkg/storage"
	"github.com/buildbarn/bb-asset-hub/pkg/proto/asset"

	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFetchBlobCaching(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := bb_digest.NewInstanceName("")
	require.NoError(t, err)

	uri := "www.example.com"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{uri},
	}
	blobDigest := &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123}
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference(uri, []*remoteasset.Qualifier{}), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	assetStore := storage.NewAssetStore(backend, 16*1024*1024)
	mockFetcher := mock.NewMockFetchServer(ctrl)
	cachingFetcher := fetch.NewCachingFetcher(mockFetcher, assetStore)

	t.Run("Success", func(t *testing.T) {
		backendGetCall := backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewBufferFromError(status.Error(codes.NotFound, "Blob not found")))
		fetchBlobCall := mockFetcher.EXPECT().FetchBlob(ctx, request).Return(&remoteasset.FetchBlobResponse{
			Status: status.New(codes.OK, "Success!").Proto(),
			Uri: uri,
			BlobDigest: blobDigest,
		}, nil).After(backendGetCall)
		backend.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
			func(ctx context.Context, digest bb_digest.Digest, b buffer.Buffer) error {
				m, err := b.ToProto(&asset.Asset{}, 1000)
				require.NoError(t, err)
				a := m.(*asset.Asset)
				require.True(t, proto.Equal(a.Digest, blobDigest))
				return nil
			}).After(fetchBlobCall)
		response, err := cachingFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		backendGetCall := backend.EXPECT().Get(ctx, gomock.Any()).Return(buffer.NewBufferFromError(status.Error(codes.NotFound, "Blob not found")))
		mockFetcher.EXPECT().FetchBlob(ctx, request).Return(nil, status.Error(codes.NotFound, "Not Found!")).After(backendGetCall)
		_, err := cachingFetcher.FetchBlob(ctx, request)
		require.NotNil(t, err)
	})

	t.Run("Cached", func(t *testing.T) {
		backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewProtoBufferFromProto(storage.NewAsset(blobDigest, nil), buffer.UserProvided))
		response, err := cachingFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}

func TestFetchDirectoryCaching(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := bb_digest.NewInstanceName("")
	require.NoError(t, err)

	uri := "www.example.com"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{uri},
	}
	dirDigest := &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123}
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference(uri, []*remoteasset.Qualifier{}), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	assetStore := storage.NewAssetStore(backend, 16*1024*1024)
	mockFetcher := mock.NewMockFetchServer(ctrl)
	cachingFetcher := fetch.NewCachingFetcher(mockFetcher, assetStore)

	t.Run("Success", func(t *testing.T) {
		backendGetCall := backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewBufferFromError(status.Error(codes.NotFound, "Directory not found")))
		fetchDirectoryCall := mockFetcher.EXPECT().FetchDirectory(ctx, request).Return(&remoteasset.FetchDirectoryResponse{
			Status: status.New(codes.OK, "Success!").Proto(),
			Uri: uri,
			RootDirectoryDigest: dirDigest,
		}, nil).After(backendGetCall)
		backend.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
			func(ctx context.Context, digest bb_digest.Digest, b buffer.Buffer) error {
				m, err := b.ToProto(&asset.Asset{}, 1000)
				require.NoError(t, err)
				a := m.(*asset.Asset)
				require.True(t, proto.Equal(a.Digest, dirDigest))
				return nil
			}).After(fetchDirectoryCall)
		response, err := cachingFetcher.FetchDirectory(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		backendGetCall := backend.EXPECT().Get(ctx, gomock.Any()).Return(buffer.NewBufferFromError(status.Error(codes.NotFound, "Directory not found")))
		mockFetcher.EXPECT().FetchDirectory(ctx, request).Return(nil, status.Error(codes.NotFound, "Not Found!")).After(backendGetCall)
		_, err := cachingFetcher.FetchDirectory(ctx, request)
		require.NotNil(t, err)
	})

	t.Run("Cached", func(t *testing.T) {
		backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewProtoBufferFromProto(storage.NewAsset(dirDigest, nil), buffer.UserProvided))
		response, err := cachingFetcher.FetchDirectory(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}
