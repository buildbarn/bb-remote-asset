package fetch_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/buildbarn/bb-remote-asset/internal/mock"
	"github.com/buildbarn/bb-remote-asset/pkg/fetch"
	bb_digest "github.com/buildbarn/bb-storage/pkg/digest"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type headerMatcher struct {
	headers map[string]string
}

func (hm *headerMatcher) String() string {
	return fmt.Sprintf("has headers: %v", hm.headers)
}

func (hm *headerMatcher) Matches(x interface{}) bool {
	req, ok := x.(*http.Request)
	if !ok {
		return false
	}

	for header, val := range hm.headers {
		headerList, ok := req.Header[header]
		if !ok {
			return false
		}

		if headerList[0] != val {
			return false
		}
	}

	return true
}

func TestHTTPFetcherFetchBlob(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	uri := "www.example.com"
	request := &remoteasset.FetchBlobRequest{
		InstanceName: "",
		Uris:         []string{uri, "www.another.com"},
		Qualifiers: []*remoteasset.Qualifier{
			{
				Name:  "checksum.sri",
				Value: "sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
			},
		},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)
	body := mock.NewMockReadCloser(ctrl)
	helloDigest := bb_digest.MustNewDigest(
		"",
		remoteexecution.DigestFunction_SHA256,
		"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969",
		5,
	)

	t.Run("Success", func(t *testing.T) {
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("SuccessNoContentLength", func(t *testing.T) {
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: -1,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		bodyCloseCall := body.EXPECT().Close().Return(nil).After(bodyReadCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyCloseCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("SuccessNoExpectedDigest", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{uri, "www.another.com"},
			Qualifiers:   []*remoteasset.Qualifier{},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		bodyCloseCall := body.EXPECT().Close().Return(nil).After(bodyReadCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyCloseCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("SuccessNoExpectedDigestOrContentLength", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{uri, "www.another.com"},
			Qualifiers:   []*remoteasset.Qualifier{},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: -1,
		}, nil)
		bodyReadCall := body.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
			copy(p, "Hello")
			return 5, io.EOF
		}).After(httpDoCall)
		bodyCloseCall := body.EXPECT().Close().Return(nil).After(bodyReadCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(bodyCloseCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("OneFailOneSuccess", func(t *testing.T) {
		httpFailCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:     "404 Not Found",
			StatusCode: 404,
		}, nil)
		httpSuccessCall := roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil).After(httpFailCall)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpSuccessCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("Failure", func(t *testing.T) {
		roundTripper.EXPECT().RoundTrip(gomock.Any()).Return(&http.Response{
			Status:     "404 Not Found",
			StatusCode: 404,
		}, nil).MaxTimes(2)

		_, err := HTTPFetcher.FetchBlob(ctx, request)
		require.NotNil(t, err)
		require.Equal(t, status.Code(err), codes.NotFound)
	})

	t.Run("WithLegacyAuthHeaders", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{uri},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "bazel.auth_headers",
					Value: `{ "www.example.com": {"Authorization": "Bearer letmein"}}`,
				},
				{
					Name:  "checksum.sri",
					Value: "sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
				},
			},
		}
		matcher := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer letmein",
			},
		}
		httpDoCall := roundTripper.EXPECT().RoundTrip(matcher).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)
		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})

	t.Run("WithAuthHeaders", func(t *testing.T) {
		request := &remoteasset.FetchBlobRequest{
			InstanceName: "",
			Uris:         []string{"www.another.com", uri},
			Qualifiers: []*remoteasset.Qualifier{
				{
					Name:  "http_header:Authorization",
					Value: `Bearer anothertoken`,
				},
				{
					Name:  "http_header:Accept",
					Value: "application/vnd.docker.distribution.manifest.list.v2+json",
				},
				{
					Name:  "http_header_url:1:Authorization",
					Value: `Bearer letmein1`,
				},
				{
					Name:  "checksum.sri",
					Value: "sha256-GF+NsyJx/iX1Yab8k4suJkMG7DBO2lGAB9F2SCY4GWk=",
				},
			},
		}
		matcherReq1 := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer anothertoken",
				"Accept":        "application/vnd.docker.distribution.manifest.list.v2+json",
			},
		}
		matcherReq2 := &headerMatcher{
			headers: map[string]string{
				"Authorization": "Bearer letmein1",
				"Accept":        "application/vnd.docker.distribution.manifest.list.v2+json",
			},
		}
		roundTripper.EXPECT().RoundTrip(matcherReq1).Return(&http.Response{
			Status:     "404 NotFound",
			StatusCode: 404,
		}, nil)
		httpDoCall2 := roundTripper.EXPECT().RoundTrip(matcherReq2).Return(&http.Response{
			Status:        "200 Success",
			StatusCode:    200,
			Body:          body,
			ContentLength: 5,
		}, nil)

		casBlobAccess.EXPECT().Put(ctx, helloDigest, gomock.Any()).Return(nil).After(httpDoCall2)

		response, err := HTTPFetcher.FetchBlob(ctx, request)
		require.Nil(t, err)
		require.True(t, proto.Equal(response.BlobDigest, helloDigest.GetProto()))
		require.Equal(t, response.Status.Code, int32(codes.OK))
	})
}

func TestHTTPFetcherFetchDirectory(t *testing.T) {
	ctrl, ctx := gomock.WithContext(context.Background(), t)

	uri := "www.example.com"
	request := &remoteasset.FetchDirectoryRequest{
		InstanceName: "",
		Uris:         []string{uri, "www.another.com"},
	}
	casBlobAccess := mock.NewMockBlobAccess(ctrl)
	roundTripper := mock.NewMockRoundTripper(ctrl)
	HTTPFetcher := fetch.NewHTTPFetcher(&http.Client{Transport: roundTripper}, casBlobAccess)
	_, err := HTTPFetcher.FetchDirectory(ctx, request)
	require.NotNil(t, err)
	require.Equal(t, status.Code(err), codes.PermissionDenied)
}
