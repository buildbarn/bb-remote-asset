package fetch_test

import (
	"context"
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFetchBlobAuthorization(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	baseFetcher := mock.NewMockFetcher(ctrl)
	authorizer := mock.NewMockAuthorizer(ctrl)
	af := fetch.NewAuthorizingFetcher(baseFetcher, authorizer)

	instanceName := bb_digest.MustNewInstanceName("gondor")
	instanceSlice := []bb_digest.InstanceName{instanceName}

	uri := "source.test"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "gondor",
		Uris:         []string{uri},
	}

	wantDigest := &remoteexecution.Digest{
		Hash:      "72fb9f5040048e3609b243329c60c580d6106ef8204194ec293a143d595607b6",
		SizeBytes: 123,
	}

	wantResp := &remoteasset.FetchBlobResponse{
		Status:     status.New(codes.OK, "Success!").Proto(),
		Uri:        uri,
		BlobDigest: wantDigest,
	}

	t.Run("Allowed", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{nil})
		baseFetcher.EXPECT().FetchBlob(ctx, request).Return(wantResp, nil)

		gotResp, err := af.FetchBlob(ctx, request)
		require.NoError(t, err)
		require.Equal(t, wantResp, gotResp)
	})

	t.Run("Rejected", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{status.Error(codes.PermissionDenied, "None shall pass")})

		_, err := af.FetchBlob(ctx, request)
		require.Equal(t, status.Error(codes.PermissionDenied, "None shall pass"), err)
	})
}

func TestFetchDirectoryAuthorization(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	baseFetcher := mock.NewMockFetcher(ctrl)
	authorizer := mock.NewMockAuthorizer(ctrl)
	af := fetch.NewAuthorizingFetcher(baseFetcher, authorizer)

	instanceName := bb_digest.MustNewInstanceName("gondor")
	instanceSlice := []bb_digest.InstanceName{instanceName}

	uri := "source.test"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "gondor",
		Uris:         []string{uri},
	}

	wantDigest := &remoteexecution.Digest{
		Hash:      "72fb9f5040048e3609b243329c60c580d6106ef8204194ec293a143d595607b6",
		SizeBytes: 123,
	}

	wantResp := &remoteasset.FetchDirectoryResponse{
		Status:              status.New(codes.OK, "Success!").Proto(),
		Uri:                 uri,
		RootDirectoryDigest: wantDigest,
	}

	t.Run("Allowed", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{nil})
		baseFetcher.EXPECT().FetchDirectory(ctx, request).Return(wantResp, nil)

		gotResp, err := af.FetchDirectory(ctx, request)
		require.NoError(t, err)
		require.Equal(t, wantResp, gotResp)
	})

	t.Run("Rejected", func(t *testing.T) {
		authorizer.EXPECT().Authorize(ctx, instanceSlice).Return([]error{status.Error(codes.PermissionDenied, "None shall pass")})

		_, err := af.FetchDirectory(ctx, request)
		require.Equal(t, status.Error(codes.PermissionDenied, "None shall pass"), err)
	})
}
