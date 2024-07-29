package storage_test

import (
	"context"
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/proto/asset"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBlobAccessAssetStorePut(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	blobDigest := &remoteexecution.Digest{Hash: "58de0f27ce0f781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643", SizeBytes: 111}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{})
	assetData := storage.NewBlobAsset(blobDigest, timestamppb.Now())
	refDigest, err := storage.AssetReferenceToDigest(assetRef, instanceName)
	require.NoError(t, err)

	blobAccess := mock.NewMockBlobAccess(ctrl)
	blobAccess.EXPECT().Put(ctx, refDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&asset.Asset{}, 1000)
			require.NoError(t, err)
			a := m.(*asset.Asset)
			require.True(t, proto.Equal(a.Digest, blobDigest))
			return nil
		})
	assetStore := storage.NewBlobAccessAssetStore(blobAccess, 16*1024*1024)

	err = assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NoError(t, err)
}

func TestBlobAccessAssetStoreGet(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("foo")
	require.NoError(t, err)

	blobDigest := &remoteexecution.Digest{Hash: "aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f", SizeBytes: 222}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{})
	refDigest, err := storage.AssetReferenceToDigest(assetRef, instanceName)
	require.NoError(t, err)

	buf := buffer.NewProtoBufferFromProto(&asset.Asset{Digest: blobDigest}, buffer.UserProvided)

	backend := mock.NewMockBlobAccess(ctrl)
	backend.EXPECT().Get(ctx, refDigest).Return(buf)
	assetStore := storage.NewBlobAccessAssetStore(backend, 16*1024*1024)

	_, err = assetStore.Get(ctx, assetRef, instanceName)
	require.NoError(t, err)
}
