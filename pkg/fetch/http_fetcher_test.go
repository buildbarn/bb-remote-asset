package fetch_test

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/buildbarn/bb-asset-hub/internal/mock"
	"github.com/buildbarn/bb-asset-hub/pkg/fetch"

	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHTTPFetcherFetchBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := bb_digest.NewInstanceName("")
	require.NoError(t, err)

	uri := "www.example.com"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{uri, "www.another.com"},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	httpClient := mock.NewMockHTTPClient(ctrl)
	allowUpdatesForInstances := map[bb_digest.InstanceName]bool{instanceName: true}
	HTTPFetcher := fetch.NewHTTPFetcher(httpClient, casBlobAccess, allowUpdatesForInstances)
	body := mock.NewMockReadCloser(ctrl)
	helloDigest := bb_digest.MustNewDigest("", "185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969", 5)

	t.Run("Success", func(t *testing.T) {
		httpDoCall := httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{
			Status:     "200 Success",
			StatusCode: 200,
			Body:       body,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyReadCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("OneFailOneSuccess", func(t *testing.T) {
		httpFailCall := httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{
			Status:     "404 Not Found",
			StatusCode: 404,
		}, nil)
		httpSuccessCall := httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{
			Status:     "200 Success",
			StatusCode: 200,
			Body:       body,
		}, nil).After(httpFailCall)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpSuccessCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyReadCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{
			Status:     "404 Not Found",
			StatusCode: 404,
		}, nil).MaxTimes(2)

		_, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NotNil(t, err)
		require.Equal(t, status.Code(err), codes.NotFound)
	})
}

func TestHTTPFetcherFetchDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	instanceName, err := bb_digest.NewInstanceName("")
	require.NoError(t, err)

	uri := "www.example.com"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{uri, "www.another.com"},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	httpClient := mock.NewMockHTTPClient(ctrl)
	allowUpdatesForInstances := map[bb_digest.InstanceName]bool{instanceName: true}
	HTTPFetcher := fetch.NewHTTPFetcher(httpClient, casBlobAccess, allowUpdatesForInstances)
	_, err = HTTPFetcher.FetchDirectory(ctx, request)
	require.NotNil(t, err)
	require.Equal(t, status.Code(err), codes.PermissionDenied)
}
