package storage_test

import (
	"context"
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/storage"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAuthorizingBlobAccessGet(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName := bb_digest.MustNewInstanceName("rohan")
	instanceSlice := []bb_digest.InstanceName{instanceName}

	blobDigest := &remoteexecution.Digest{Hash: "b27cad931e1ef0a520887464127055ffd6db82c7b36bfea5cd832db65b8f816b", SizeBytes: 24}
	uri := "https://raapi.test/blob"
	assetRef := storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{})
	assetData := storage.NewBlobAsset(blobDigest, timestamppb.Now())

	baseStore := mock.NewMockAssetStore(ctrl)
	fetchAuthorizer := mock.NewMockAuthorizer(ctrl)
	pushAuthorizer := mock.NewMockAuthorizer(ctrl)
	aas := storage.NewAuthorizingAssetStore(baseStore, fetchAuthorizer, pushAuthorizer)

	t.Run("Allowed", func(t *testing.T) {
		fetchAuthorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{nil})
		baseStore.EXPECT().Get(ctx, assetRef, instanceName).Return(assetData, nil)

		gotAsset, err := aas.Get(ctx, assetRef, instanceName)
		require.NoError(t, err)
		require.Equal(t, assetData, gotAsset)
	})

	t.Run("Rejected", func(t *testing.T) {
		fetchAuthorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{status.Error(codes.PermissionDenied, "None shall pass")})

		_, err := aas.Get(ctx, assetRef, instanceName)
		require.Equal(t, status.Error(codes.PermissionDenied, "None shall pass"), err)
	})
}

func TestAuthorizingBlobAccessPut(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName := bb_digest.MustNewInstanceName("rohan")
	instanceSlice := []bb_digest.InstanceName{instanceName}

	blobDigest := &remoteexecution.Digest{Hash: "b27cad931e1ef0a520887464127055ffd6db82c7b36bfea5cd832db65b8f816b", SizeBytes: 24}
	uri := "https://raapi.test/blob"
	assetRef := storage.NewAssetReference([]string{uri}, []*remoteasset.Qualifier{})
	assetData := storage.NewBlobAsset(blobDigest, timestamppb.Now())

	baseStore := mock.NewMockAssetStore(ctrl)
	fetchAuthorizer := mock.NewMockAuthorizer(ctrl)
	pushAuthorizer := mock.NewMockAuthorizer(ctrl)
	aas := storage.NewAuthorizingAssetStore(baseStore, fetchAuthorizer, pushAuthorizer)

	t.Run("Allowed", func(t *testing.T) {
		pushAuthorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{nil})
		baseStore.EXPECT().Put(ctx, assetRef, assetData, instanceName).Return(nil)

		err := aas.Put(ctx, assetRef, assetData, instanceName)
		require.NoError(t, err)
	})

	t.Run("Rejected", func(t *testing.T) {
		pushAuthorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{status.Error(codes.PermissionDenied, "None shall pass")})

		err := aas.Put(ctx, assetRef, assetData, instanceName)
		require.Equal(t, status.Error(codes.PermissionDenied, "None shall pass"), err)
	})
}
