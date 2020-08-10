package fetch_test

import (
	"context"
	"testing"

	"github.com/buildbarn/bb-asset-hub/pkg/fetch"
	"github.com/buildbarn/bb-asset-hub/internal/mock"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"

	"github.com/stretchr/testify/require"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFetchBlobUriRequirement(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	uri := "https://example.com/example.txt"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{uri},
	}
	badRequest := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:          []string{},
	}
	mockFetcher := mock.NewMockFetchServer(ctrl)

	validatingFetcher := fetch.NewValidatingFetcher(mockFetcher)

	t.Run("Success", func(t *testing.T) {
		mockFetcher.EXPECT().FetchBlob(ctx, request).Return(&remoteasset.FetchBlobResponse{
			Status: status.New(codes.OK, "Success!").Proto(),
			Uri: uri,
			BlobDigest: &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123},
		}, nil)
		response, err := validatingFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		_, err := validatingFetcher.FetchBlob(ctx, badRequest)
		require.NotNil(t, err)
		require.Equal(t, status.Code(err), codes.InvalidArgument)
	})
}

func TestFetchDirectoryUriRequirement(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	uri := "https://example.com/example.txt"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{uri},
	}
	badRequest := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{},
	}
	mockFetcher := mock.NewMockFetchServer(ctrl)

	validatingFetcher := fetch.NewValidatingFetcher(mockFetcher)

	t.Run("Success", func(t *testing.T) {
		mockFetcher.EXPECT().FetchDirectory(ctx, request).Return(&remoteasset.FetchDirectoryResponse{
			Status: status.New(codes.OK, "Success!").Proto(),
			Uri: uri,
			RootDirectoryDigest: &remoteexecution.Digest{Hash: "d0d829c4c0ce64787cb1c998a9c29a109f8ed005633132fda4f29982487b04db", SizeBytes: 123},
		}, nil)
		response, err := validatingFetcher.FetchDirectory(ctx, request)
		require.Nil(t, err)
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		_, err := validatingFetcher.FetchDirectory(ctx, badRequest)
		require.NotNil(t, err)
		require.Equal(t, status.Code(err), codes.InvalidArgument)
	})
}
