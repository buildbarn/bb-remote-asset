package fetch_test

import (
	"context"
	"testing"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	protostatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{}), instanceName)
	require.NoError(t, err)

	t.Logf("Ref digest was %v", refDigest)

	backend := mock.NewMockBlobAccess(ctrl)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	mockFetcher := mock.NewMockFetcher(ctrl)
	cachingFetcher := fetch.NewCachingFetcher(mockFetcher, assetStore)

	t.Run("Success", func(t *testing.T) {
		backendGetCall := backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewBufferFromError(status.Error(codes.NotFound, "Blob not found")))
		fetchBlobCall := mockFetcher.EXPECT().FetchBlob(ctx, request).Return(&remoteasset.FetchBlobResponse{
			Status:     status.New(codes.OK, "Success!").Proto(),
			Uri:        uri,
			BlobDigest: blobDigest,
		}, nil).After(backendGetCall)
		backend.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
			func(ctx context.Context, digest bb_digest.Digest, b buffer.Buffer) error {
				m, err := b.ToProto(&asset.Asset{}, 1000)
				require.NoError(t, err)
				a := m.(*asset.Asset)
				require.True(t, proto.Equal(a.Digest, blobDigest))
				require.Equal(t, asset.Asset_BLOB, a.Type)
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
		backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewProtoBufferFromProto(storage.NewBlobAsset(blobDigest, nil), buffer.UserProvided))
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
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{}), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	mockFetcher := mock.NewMockFetcher(ctrl)
	cachingFetcher := fetch.NewCachingFetcher(mockFetcher, assetStore)

	t.Run("Success", func(t *testing.T) {
		backendGetCall := backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewBufferFromError(status.Error(codes.NotFound, "Directory not found")))
		fetchDirectoryCall := mockFetcher.EXPECT().FetchDirectory(ctx, request).Return(&remoteasset.FetchDirectoryResponse{
			Status:              status.New(codes.OK, "Success!").Proto(),
			Uri:                 uri,
			RootDirectoryDigest: dirDigest,
		}, nil).After(backendGetCall)
		backend.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
			func(ctx context.Context, digest bb_digest.Digest, b buffer.Buffer) error {
				m, err := b.ToProto(&asset.Asset{}, 1000)
				require.NoError(t, err)
				a := m.(*asset.Asset)
				require.True(t, proto.Equal(a.Digest, dirDigest))
				require.Equal(t, asset.Asset_DIRECTORY, a.Type)
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
		backend.EXPECT().Get(ctx, refDigest).Return(buffer.NewProtoBufferFromProto(storage.NewBlobAsset(dirDigest, nil), buffer.UserProvided))
		response, err := cachingFetcher.FetchDirectory(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}

func TestCachingFetcherExpiry(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("foo")
	require.NoError(t, err)

	uri := "https://example.com/example.tar.gz"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "foo",
		Uris:         []string{uri},
	}
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{}), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	buf := buffer.NewProtoBufferFromProto(&asset.Asset{
		Digest: &remoteexecution.Digest{
			Hash:      "d1bc8d3ba4afc7e109612cb73acbdddac052c93025aa1f82942edabb7deb82a1",
			SizeBytes: 121,
		},
		ExpireAt:    timestamppb.Now(),
		LastUpdated: timestamppb.Now(),
	}, buffer.UserProvided)
	backend.EXPECT().Get(ctx, refDigest).Return(buf)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	baseFetcher := fetch.NewErrorFetcher(&protostatus.Status{
		Code:    5,
		Message: "Not found",
	})
	cacheFetcher := fetch.NewCachingFetcher(baseFetcher, assetStore)

	_, err = cacheFetcher.FetchBlob(ctx, request)
	require.Equal(t, status.ErrorProto(&protostatus.Status{Code: 5, Message: "Not found"}), err)
}

func TestCachingFetcherOldestContentAccepted(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("bar")
	require.NoError(t, err)

	uri := "https://example.com/exampleblob.zip"
	request := &remoteasset.FetchBlobRequest{
		InstanceName:          "bar",
		Uris:                  []string{uri},
		OldestContentAccepted: timestamppb.Now(),
	}
	refDigest, err := storage.AssetReferenceToDigest(storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{}), instanceName)
	require.NoError(t, err)

	backend := mock.NewMockBlobAccess(ctrl)
	ts := timestamppb.New(time.Unix(1, 1))
	buf := buffer.NewProtoBufferFromProto(&asset.Asset{
		Digest: &remoteexecution.Digest{
			Hash:      "ad84ffc44bab3f84fc3396b4678c1fd39770fa373c3f14eedc5d60e648067960",
			SizeBytes: 234,
		},
		LastUpdated: ts,
		Type:        asset.Asset_BLOB,
	}, buffer.UserProvided)
	backend.EXPECT().Get(ctx, refDigest).Return(buf)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)
	baseFetcher := fetch.NewErrorFetcher(&protostatus.Status{
		Code:    5,
		Message: "Not found",
	})
	cacheFetcher := fetch.NewCachingFetcher(baseFetcher, assetStore)

	_, err = cacheFetcher.FetchBlob(ctx, request)
	require.Equal(t, status.ErrorProto(&protostatus.Status{Code: 5, Message: "Not found"}), err)
}
