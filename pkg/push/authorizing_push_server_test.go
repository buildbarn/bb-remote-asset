package push_test

import (
	"context"
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/push"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPushBlobAuthorization(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	basePushServer := mock.NewMockPushServer(ctrl)
	authorizer := mock.NewMockAuthorizer(ctrl)
	ap := push.NewAuthorizingPushServer(basePushServer, authorizer)

	instanceName := util.Must(bb_digest.NewInstanceName("ithilien"))
	instanceSlice := []bb_digest.InstanceName{instanceName}

	uri := "source.test"
	blobDigest := &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123}
	request := &remoteasset.PushBlobRequest{
		InstanceName: "ithilien",
		Uris:         []string{uri},
		BlobDigest:   blobDigest,
	}

	wantResp := &remoteasset.PushBlobResponse{}

	t.Run("Allowed", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{nil})
		basePushServer.EXPECT().PushBlob(ctx, request).Return(wantResp, nil)

		gotResp, err := ap.PushBlob(ctx, request)
		require.NoError(t, err)
		require.Equal(t, wantResp, gotResp)
	})

	t.Run("Rejected", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{status.Error(codes.PermissionDenied, "You shall not pass")})

		_, err := ap.PushBlob(ctx, request)
		require.Equal(t, err, status.Error(codes.PermissionDenied, "You shall not pass"))
	})
}

func TestPushDirectoryAuthorization(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	basePushServer := mock.NewMockPushServer(ctrl)
	authorizer := mock.NewMockAuthorizer(ctrl)
	ap := push.NewAuthorizingPushServer(basePushServer, authorizer)

	instanceName := util.Must(bb_digest.NewInstanceName("ithilien"))
	instanceSlice := []bb_digest.InstanceName{instanceName}

	uri := "source.test"
	directoryDigest := &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123}
	request := &remoteasset.PushDirectoryRequest{
		InstanceName:        "ithilien",
		Uris:                []string{uri},
		RootDirectoryDigest: directoryDigest,
	}

	wantResp := &remoteasset.PushDirectoryResponse{}

	t.Run("Allowed", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{nil})
		basePushServer.EXPECT().PushDirectory(ctx, request).Return(wantResp, nil)

		gotResp, err := ap.PushDirectory(ctx, request)
		require.NoError(t, err)
		require.Equal(t, wantResp, gotResp)
	})

	t.Run("Rejected", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{status.Error(codes.PermissionDenied, "You shall not pass")})

		_, err := ap.PushDirectory(ctx, request)
		require.Equal(t, err, status.Error(codes.PermissionDenied, "You shall not pass"))
	})
}
