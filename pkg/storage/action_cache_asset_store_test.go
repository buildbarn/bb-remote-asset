package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func bbDigest(d *remoteexecution.Digest) digest.Digest {
	return digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		d.Hash,
		d.SizeBytes,
	)
}

func TestActionCacheAssetStorePutBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName := digest.MustNewInstanceName("")

	blobDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce0f781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 111,
	}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewBlobAsset(blobDigest, timestamppb.Now())
	refDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"a2c2b32a289d4d9bf6e6309ed2691b6bcc04ee7923fcfd81bf1bfe0e7348139b",
		14,
	)
	directoryDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"c72e5e1e6ab54746d4fd3da7b443037187c81347a210d2ab8e5863638fbe1ac6",
		88,
	)
	actionDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"80fa2440711e986859b84bb5bc5f63b3a9987aa498a4019824a7bed622593f6e",
		140,
	)
	commandDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"3c169433e9fad318ccf601d327685f95941ce93408fc0d21f92452844564d123",
		40,
	)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	cas.EXPECT().Put(ctx, refDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, directoryDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, actionDigest, gomock.Any()).Return(nil)
	cas.EXPECT().Put(ctx, commandDigest, gomock.Any()).Return(nil)
	ac.EXPECT().Put(ctx, actionDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&remoteexecution.ActionResult{}, 1000)
			require.NoError(t, err)
			a := m.(*remoteexecution.ActionResult)
			for _, f := range a.OutputFiles {
				if f.Path == "out" {
					require.True(t, proto.Equal(f.Digest, blobDigest), "Got %v", f.Digest)
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
	bbRootDirectoryDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		rootDirectoryDigest.Hash,
		rootDirectoryDigest.SizeBytes,
	)
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewAsset(
		rootDirectoryDigest,
		asset.Asset_DIRECTORY,
		timestamppb.Now(),
	)
	refDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"a2c2b32a289d4d9bf6e6309ed2691b6bcc04ee7923fcfd81bf1bfe0e7348139b",
		14,
	)
	directoryDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"c72e5e1e6ab54746d4fd3da7b443037187c81347a210d2ab8e5863638fbe1ac6",
		88,
	)
	actionDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"80fa2440711e986859b84bb5bc5f63b3a9987aa498a4019824a7bed622593f6e",
		140,
	)
	commandDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"3c169433e9fad318ccf601d327685f95941ce93408fc0d21f92452844564d123",
		40,
	)
	bbTreeDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"102b51b9765a56a3e899f7cf0ee38e5251f9c503b357b330a49183eb7b155604",
		2,
	)
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
					require.True(t, proto.Equal(d.TreeDigest, treeDigest), "Got %v", d.TreeDigest)
					require.True(t, proto.Equal(d.RootDirectoryDigest, rootDirectoryDigest), "Got %v", d.RootDirectoryDigest)
					return nil
				}
			}
			return status.Error(codes.Internal, "Directory digest not found")
		})
	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	err = assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NoError(t, err)
}

func TestActionCacheAssetStorePutMalformedDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	rootDirectoryDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce0f781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 111,
	}
	bbRootDirectoryDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		rootDirectoryDigest.Hash,
		rootDirectoryDigest.SizeBytes,
	)
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewAsset(
		rootDirectoryDigest,
		asset.Asset_DIRECTORY,
		timestamppb.Now(),
	)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	cas.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	cas.EXPECT().Get(ctx, bbRootDirectoryDigest).Return(
		buffer.NewProtoBufferFromProto(&remoteexecution.Directory{
			Directories: []*remoteexecution.DirectoryNode{{
				Name:   "this is a malformed directory noe",
				Digest: nil,
			}},
		},
			buffer.UserProvided))

	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	err = assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NotNil(t, err)
}

func TestActionCacheAssetStorePutRecursiveDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	sub1Digest := &remoteexecution.Digest{
		Hash:      "94a72d7ae68d937c7d65ccc7310a97a11ce78a48850ff618fcbeba58c354e07d",
		SizeBytes: 40,
	}
	sub2Digest := &remoteexecution.Digest{
		Hash:      "1dc3fa2e0703bb64c17a3b0b4402c44a666ec8ac361e77bb526a65dea6d73bf0",
		SizeBytes: 0,
	}

	tree := &remoteexecution.Tree{
		Root: &remoteexecution.Directory{
			Directories: []*remoteexecution.DirectoryNode{
				{
					Name:   "sub1",
					Digest: sub1Digest,
				},
				{
					Name:   "sub2",
					Digest: sub2Digest,
				},
			},
			Files: []*remoteexecution.FileNode{
				{
					Digest: &remoteexecution.Digest{
						Hash:      "593dd41a19cddee5a67a5bcde0d2323199cc340fa64d6c24a22c5913960a6de2",
						SizeBytes: 6,
					},
				},
			},
		},
		Children: []*remoteexecution.Directory{
			{
				Files: []*remoteexecution.FileNode{
					{
						Digest: &remoteexecution.Digest{
							Hash:      "12ca9a458e433b707f72d00c2aa659529fee2b9e97de9fe645281b3a13ac6ee9",
							SizeBytes: 5,
						},
					},
				},
			},
			{},
		},
	}

	rootDirectoryDigest, err := storage.ProtoToDigest(tree.Root)
	require.NoError(t, err)
	treeDigest, err := storage.ProtoToDigest(tree)
	require.NoError(t, err)

	t.Logf("rootDirectoryDigest %v", rootDirectoryDigest)
	t.Logf("treeDigest %v", treeDigest)

	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewAsset(
		rootDirectoryDigest,
		asset.Asset_DIRECTORY,
		timestamppb.Now(),
	)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	cas.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	cas.EXPECT().Get(ctx, bbDigest(rootDirectoryDigest)).Return(
		buffer.NewProtoBufferFromProto(
			tree.Root,
			buffer.UserProvided,
		))
	cas.EXPECT().Get(ctx, bbDigest(sub1Digest)).Return(
		buffer.NewProtoBufferFromProto(
			tree.Children[0],
			buffer.UserProvided,
		))
	cas.EXPECT().Get(ctx, bbDigest(sub2Digest)).Return(
		buffer.NewProtoBufferFromProto(
			tree.Children[1],
			buffer.UserProvided,
		))

	ac.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&remoteexecution.ActionResult{}, 1000)
			require.NoError(t, err)
			a := m.(*remoteexecution.ActionResult)
			for _, d := range a.OutputDirectories {
				if d.Path == "out" {
					require.True(t, proto.Equal(d.TreeDigest, treeDigest), "Got %v", d.TreeDigest)
					require.True(t, proto.Equal(d.RootDirectoryDigest, rootDirectoryDigest), "Got %v", d.RootDirectoryDigest)
					return nil
				}
			}
			return status.Error(codes.Internal, "Directory digest not found")
		})

	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	err = assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NoError(t, err)
}

func TestActionCacheAssetStorePutMalformedDirectoryAsBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	blobDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce0f781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 111,
	}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})
	assetData := storage.NewAsset(
		blobDigest,
		asset.Asset_BLOB,
		timestamppb.Now(),
	)
	actionDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"80fa2440711e986859b84bb5bc5f63b3a9987aa498a4019824a7bed622593f6e",
		140,
	)

	ac := mock.NewMockBlobAccess(ctrl)
	cas := mock.NewMockBlobAccess(ctrl)
	cas.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).Return(nil).Times(4)
	ac.EXPECT().Put(ctx, actionDigest, gomock.Any()).DoAndReturn(
		func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
			m, err := b.ToProto(&remoteexecution.ActionResult{}, 1000)
			require.NoError(t, err)
			a := m.(*remoteexecution.ActionResult)
			for _, d := range a.OutputFiles {
				if d.Path == "out" {
					require.True(t, proto.Equal(d.Digest, blobDigest), "Got %v", d.Digest)
					return nil
				}
			}
			return status.Error(codes.Internal, "Directory digest not found")
		})
	assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

	err = assetStore.Put(ctx, assetRef, assetData, instanceName)
	require.NoError(t, err)
}

func roundTripTest(t *testing.T, assetRef *asset.AssetReference, assetData *asset.Asset) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := digest.NewInstanceName("")
	require.NoError(t, err)

	var actionDigest digest.Digest
	var actionResult *remoteexecution.ActionResult

	{
		ac := mock.NewMockBlobAccess(ctrl)
		cas := mock.NewMockBlobAccess(ctrl)

		cas.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).AnyTimes()

		if assetData.Type == asset.Asset_DIRECTORY {
			cas.EXPECT().Get(ctx, gomock.Any()).Return(
				buffer.NewProtoBufferFromProto(&remoteexecution.Directory{},
					buffer.UserProvided))
		}

		ac.EXPECT().Put(ctx, gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, digest digest.Digest, b buffer.Buffer) error {
				actionDigest = digest
				m, err := b.ToProto(&remoteexecution.ActionResult{}, 1000)
				require.NoError(t, err)
				actionResult = m.(*remoteexecution.ActionResult)
				return nil
			})

		assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

		err = assetStore.Put(ctx, assetRef, assetData, instanceName)
		require.NoError(t, err)
	}
	{
		require.NotNil(t, actionResult)

		ac := mock.NewMockBlobAccess(ctrl)
		cas := mock.NewMockBlobAccess(ctrl)

		ac.EXPECT().Get(ctx, gomock.Any()).DoAndReturn(
			func(ctx context.Context, digest digest.Digest) buffer.Buffer {
				if digest == actionDigest {
					return buffer.NewProtoBufferFromProto(actionResult, buffer.UserProvided)
				}
				return buffer.NewBufferFromError(fmt.Errorf("not in AC"))
			})

		assetStore := storage.NewActionCacheAssetStore(ac, cas, 16*1024*1024)

		asset, err := assetStore.Get(ctx, assetRef, instanceName)
		require.NoError(t, err)
		require.Equal(t, asset.Digest, assetData.Digest)
	}
}

func TestActionCacheAssetStoreRoundTrip(t *testing.T) {
	expectedDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce00781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 115,
	}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})

	assetData := storage.NewBlobAsset(expectedDigest, timestamppb.Now())

	roundTripTest(t, assetRef, assetData)
}

func TestActionCacheAssetStoreRoundTripDirectory(t *testing.T) {
	expectedDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce00781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 115,
	}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "test", Value: "test"}})

	assetData := storage.NewAsset(
		expectedDigest,
		asset.Asset_DIRECTORY,
		timestamppb.Now(),
	)

	roundTripTest(t, assetRef, assetData)
}

func TestActionCacheAssetStoreRoundTripWithSpecialQualifiers(t *testing.T) {
	expectedDigest := &remoteexecution.Digest{
		Hash:      "58de0f27ce00781e5c109f18b0ee6905bdf64f2b1009e225ac67a27f656a0643",
		SizeBytes: 115,
	}
	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{{Name: "resource_type", Value: "application/x-git"}})

	assetData := storage.NewBlobAsset(expectedDigest, timestamppb.Now())

	roundTripTest(t, assetRef, assetData)
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
	actionDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"7bef991fed17d0a31d1ea1b536f2ac865e567e34ebfa2bfc081d3672110b93be",
		140,
	)

	ts := timestamppb.New(time.Unix(0, 0))
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

	uri := "https://example.com/example.txt"
	assetRef := storage.NewAssetReference([]string{uri},
		[]*remoteasset.Qualifier{})
	actionDigest := digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"7bef991fed17d0a31d1ea1b536f2ac865e567e34ebfa2bfc081d3672110b93be",
		140,
	)

	ts := timestamppb.New(time.Unix(0, 0))
	buf := buffer.NewProtoBufferFromProto(&remoteexecution.ActionResult{
		OutputFiles: []*remoteexecution.OutputFile{
			{
				Path: "out",
				Digest: &remoteexecution.Digest{
					Hash:      "aec070645fe53ee3b3763059376134f058cc337247c978add178b6ccdfb0019f",
					SizeBytes: 222,
				},
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
