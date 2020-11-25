package storage_test

import (
	"context"
	"testing"
	"time"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	"github.com/buildbarn/bb-storage/pkg/blobstore/buffer"
	"github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestActionCacheAssetStorePutBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName := digest.MustNewInstanceName("")

	blobDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce0f781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 111,
	}
	bbBlobDigest := digest.MustNewDigest("", blobDigest.Hash,
		blobDigest.SizeBytes)
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewAsset(blobDigest, ptypes.TimestampNow())
	refDigest := digest.MustNewDigest("",
		"a2c2b32a289d4d9bf6e6309ed2691b6bcc04ee7923fcfd81bf1bfe0e7348139b",
		14)
	directoryDigest := digest.MustNewDigest("",
		"c72e5e1e6ab54746d4fd3da7b443037187c81347a210d2ab8e5863638fbe1ac6",
		88)
	actionDigest := digest.MustNewDigest("",
		"ae2ece643d2907102b1949f00721514cdda44ce7cb2c03ccd2af4dac45792d09",
		140)
	commandDigest := digest.MustNewDigest("",
		"e6842def39984b212641b9796c162b9e3085da84257bae614418f2255b0addc5",
		38)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	cas.EXPECT().Put(ctx, refDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, directoryDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, actionDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, commandDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Get(ctx, bbBlobDigest).Return(
		buffer.NewValidatedBufferFromByteSlice([]byte("Hello")))
	ac.EXPECT().Put(ctx, actionDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&remoteexecution.ActionResult{}, 1000)
			require.NoError(t, err)
			a := m.(*remoteexecution.ActionResult)
			for _, f := range a.OutputFiles {
				if f.Path == "out" {
					require.True(t, proto.Equal(f.Digest, blobDigest))
					return nil
				}
			}
			return status.Error(codes.Internal, "Blob digest not found")
		})
	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	err := assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NoError(t, err)
}

func TestActionCacheAssetStorePutDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	rootDirectoryDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce0f781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 111,
	}
	bbRootDirectoryDigest := digest.MustNewDigest("",
		rootDirectoryDigest.Hash, rootDirectoryDigest.SizeBytes)
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewAsset(rootDirectoryDigest,
		ptypes.TimestampNow())
	refDigest := digest.MustNewDigest("",
		"a2c2b32a289d4d9bf6e6309ed2691b6bcc04ee7923fcfd81bf1bfe0e7348139b",
		14)
	directoryDigest := digest.MustNewDigest("",
		"c72e5e1e6ab54746d4fd3da7b443037187c81347a210d2ab8e5863638fbe1ac6",
		88)
	actionDigest := digest.MustNewDigest("",
		"ae2ece643d2907102b1949f00721514cdda44ce7cb2c03ccd2af4dac45792d09",
		140)
	commandDigest := digest.MustNewDigest("",
		"e6842def39984b212641b9796c162b9e3085da84257bae614418f2255b0addc5",
		38)
	bbTreeDigest := digest.MustNewDigest("",
		"102b51b9765a56a3e899f7cf0ee38e5251f9c503b357b330a49183eb7b155604", 2)
	treeDigest := &remoteexecution.Digest{
		Hash:      "102b51b9765a56a3e899f7cf0ee38e5251f9c503b357b330a49183eb7b155604",
		SizeBytes: 2,
	}

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	cas.EXPECT().Put(ctx, refDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, directoryDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, actionDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, commandDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Get(ctx, bbRootDirectoryDigest).Return(
		buffer.NewProtoBufferFromProto(&remoteexecution.Directory{},
			buffer.UserProvided))
	cas.EXPECT().Put(ctx, bbTreeDigest, gomock.Any()).Return(nil)
	ac.EXPECT().Put(ctx, actionDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&remoteexecution.ActionResult{}, 1000)
			require.NoError(t, err)
			a := m.(*remoteexecution.ActionResult)
			for _, d := range a.OutputDirectories {
				if d.Path == "out" {
					require.True(t, proto.Equal(d.TreeDigest, treeDigest))
					return nil
				}
			}
			return status.Error(codes.Internal, "Directory digest not found")
		})
	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	err = assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NoError(t, err)
}

func TestActionCacheAssetStoreGetBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName := digest.MustNewInstanceName("")

	blobDigest := &remoteexecution.Digest{
		Hash:      "aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f",
		SizeBytes: 222,
	}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{})
	actionDigest := digest.MustNewDigest("",
		"1543af664d856ac553f43cca0f61b3b948bafd6802308d67f42bbc09cd042218",
		140)

	ts, _ := ptypes.TimestampProto(time.Unix(0, 0))
	buf := buffer.NewProtoBufferFromProto(&remoteexecution.ActionResult{
		OutputFiles: []*remoteexecution.OutputFile{
			{
				Path:   "out",
				Digest: blobDigest,
			},
		},
		ExecutionMetadata: &remoteexecution.ExecutedActionMetadata{
			QueuedTimestamp: ts,
		},
	}, buffer.UserProvided)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	ac.EXPECT().Get(ctx, actionDigest).Return(buf)
	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	_, err := assetStore.Get(ctx, assetRef, instanceName)
	require.NoError(t, err)
}

func TestActionCacheAssetStoreGetDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName := digest.MustNewInstanceName("")

	treeDigest := &remoteexecution.Digest{
		Hash:      "aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f",
		SizeBytes: 222,
	}
	bbTreeDigest := digest.MustNewDigest("",
		"aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f",
		222)
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{})
	actionDigest := digest.MustNewDigest("",
		"1543af664d856ac553f43cca0f61b3b948bafd6802308d67f42bbc09cd042218",
		140)

	ts, _ := ptypes.TimestampProto(time.Unix(0, 0))
	buf := buffer.NewProtoBufferFromProto(&remoteexecution.ActionResult{
		OutputDirectories: []*remoteexecution.OutputDirectory{
			{
				Path:       "out",
				TreeDigest: treeDigest,
			},
		},
		ExecutionMetadata: &remoteexecution.ExecutedActionMetadata{
			QueuedTimestamp: ts,
		},
	}, buffer.UserProvided)

	treeBuf := buffer.NewProtoBufferFromProto(&remoteexecution.Tree{
		Root: &remoteexecution.Directory{},
	}, buffer.UserProvided)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	ac.EXPECT().Get(ctx, actionDigest).Return(buf)
	cas.EXPECT().Get(ctx, bbTreeDigest).Return(treeBuf)
	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	_, err := assetStore.Get(ctx, assetRef, instanceName)
	require.NoError(t, err)
}
